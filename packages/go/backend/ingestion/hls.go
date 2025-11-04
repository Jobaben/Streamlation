package ingestion

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HLSConfig tunes behaviour of the HLS stream source.
type HLSConfig struct {
	PlaylistURL     string
	Client          *http.Client
	PollInterval    time.Duration
	BufferSize      int
	RetryBackoff    time.Duration
	MaxRetryBackoff time.Duration
	MaxSeenSegments int
}

// NewHLSStreamSource constructs a StreamSource that pulls media chunks from an HLS playlist.
func NewHLSStreamSource(cfg HLSConfig) (*HLSStreamSource, error) {
	if cfg.PlaylistURL == "" {
		return nil, errors.New("playlist URL is required")
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 8
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 500 * time.Millisecond
	}
	if cfg.MaxRetryBackoff <= 0 {
		cfg.MaxRetryBackoff = 5 * time.Second
	}
	if cfg.MaxSeenSegments <= 0 {
		cfg.MaxSeenSegments = 256
	}
	playlistURL, err := url.Parse(cfg.PlaylistURL)
	if err != nil {
		return nil, fmt.Errorf("invalid playlist URL: %w", err)
	}
	return &HLSStreamSource{
		cfg:         cfg,
		playlistURL: playlistURL,
		counters:    &streamCounters{},
	}, nil
}

// HLSStreamSource implements StreamSource for HTTP Live Streaming playlists.
type HLSStreamSource struct {
	cfg         HLSConfig
	playlistURL *url.URL
	counters    *streamCounters
}

// Stream starts polling the playlist and emits newly discovered segments.
func (s *HLSStreamSource) Stream(ctx context.Context) (<-chan MediaChunk, <-chan error) {
	chunks := make(chan MediaChunk, s.cfg.BufferSize)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		client := s.cfg.Client
		seenSegments := make(map[string]int64)
		backoff := s.cfg.RetryBackoff
		var seenCounter int64
		maxSeen := s.cfg.MaxSeenSegments

		for {
			if ctx.Err() != nil {
				return
			}

			segments, err := s.fetchSegments(ctx, client)
			if err != nil {
				s.counters.errors.Add(1)
				select {
				case errs <- err:
				default:
				}
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return
				}
				if next := backoff * 2; next <= s.cfg.MaxRetryBackoff {
					backoff = next
				}
				s.counters.reconnect.Add(1)
				continue
			}

			backoff = s.cfg.RetryBackoff
			for _, seg := range segments {
				if _, seen := seenSegments[seg.uri]; seen {
					continue
				}
				seenCounter++
				seenSegments[seg.uri] = seenCounter
				if len(seenSegments) > maxSeen {
					threshold := seenCounter - int64(maxSeen)
					for uri, seq := range seenSegments {
						if seq <= threshold {
							delete(seenSegments, uri)
						}
					}
				}

				data, err := s.downloadSegment(ctx, client, seg.uri)
				if err != nil {
					s.counters.errors.Add(1)
					delete(seenSegments, seg.uri)
					select {
					case errs <- err:
					default:
					}
					continue
				}

				chunk := MediaChunk{
					Sequence:  s.counters.sequence.Add(1),
					Timestamp: time.Now().UTC(),
					Duration:  seg.duration,
					Payload:   data,
					Metadata: map[string]string{
						"uri": seg.uri,
					},
				}

				select {
				case chunks <- chunk:
					s.counters.received.Add(1)
				default:
					s.counters.dropped.Add(1)
				}
			}

			select {
			case <-time.After(s.cfg.PollInterval):
			case <-ctx.Done():
				return
			}
		}
	}()

	return chunks, errs
}

// Metrics returns the current counters snapshot.
func (s *HLSStreamSource) Metrics() StreamMetrics {
	return s.counters.snapshot()
}

type hlsSegment struct {
	uri      string
	duration time.Duration
}

func (s *HLSStreamSource) fetchSegments(ctx context.Context, client *http.Client) ([]hlsSegment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.PlaylistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build playlist request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch playlist: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("playlist returned %s", resp.Status)
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read playlist: %w", err)
	}
	return s.parsePlaylist(buf)
}

func (s *HLSStreamSource) downloadSegment(ctx context.Context, client *http.Client, segmentURI string) ([]byte, error) {
	uri, err := s.playlistURL.Parse(segmentURI)
	if err != nil {
		return nil, fmt.Errorf("resolve segment URI: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build segment request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch segment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("segment returned %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read segment: %w", err)
	}
	return data, nil
}

func (s *HLSStreamSource) parsePlaylist(body []byte) ([]hlsSegment, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Split(bufio.ScanLines)

	var (
		segments        []hlsSegment
		pendingDuration time.Duration
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXTINF:") {
			duration, err := parseDuration(line)
			if err != nil {
				return nil, err
			}
			pendingDuration = duration
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		segments = append(segments, hlsSegment{
			uri:      line,
			duration: pendingDuration,
		})
		pendingDuration = 0
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse playlist: %w", err)
	}
	return segments, nil
}

func parseDuration(line string) (time.Duration, error) {
	value := strings.TrimPrefix(line, "#EXTINF:")
	comma := strings.IndexByte(value, ',')
	if comma >= 0 {
		value = value[:comma]
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("missing EXTINF duration")
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid EXTINF duration %q: %w", value, err)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}
