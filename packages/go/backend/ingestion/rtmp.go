package ingestion

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"time"
)

// RTMPConfig configures the RTMP stream source.
type RTMPConfig struct {
	URL            string
	Dialer         *net.Dialer
	BufferSize     int
	ReconnectDelay time.Duration
	ReadTimeout    time.Duration
}

// NewRTMPStreamSource constructs an RTMP adapter emitting MediaChunks.
func NewRTMPStreamSource(cfg RTMPConfig) (*RTMPStreamSource, error) {
	if cfg.URL == "" {
		return nil, errors.New("rtmp url is required")
	}
	parsed, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid rtmp url: %w", err)
	}
	if parsed.Scheme != "rtmp" {
		return nil, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if cfg.Dialer == nil {
		cfg.Dialer = &net.Dialer{Timeout: 5 * time.Second}
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 8
	}
	if cfg.ReconnectDelay <= 0 {
		cfg.ReconnectDelay = 500 * time.Millisecond
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 3 * time.Second
	}
	return &RTMPStreamSource{
		cfg:      cfg,
		url:      parsed,
		counters: &streamCounters{},
	}, nil
}

// RTMPStreamSource consumes a simplified RTMP-like TCP stream.
type RTMPStreamSource struct {
	cfg      RTMPConfig
	url      *url.URL
	counters *streamCounters
}

const handshakeMagic = "STRM1"

// Stream connects to the RTMP endpoint and emits framed payloads.
func (s *RTMPStreamSource) Stream(ctx context.Context) (<-chan MediaChunk, <-chan error) {
	chunks := make(chan MediaChunk, s.cfg.BufferSize)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		for {
			if ctx.Err() != nil {
				return
			}

			conn, err := s.dial(ctx)
			if err != nil {
				s.counters.errors.Add(1)
				select {
				case errs <- err:
				default:
				}
				select {
				case <-time.After(s.cfg.ReconnectDelay):
				case <-ctx.Done():
					return
				}
				continue
			}

			s.counters.reconnect.Add(1)
			if err := s.handshake(conn); err != nil {
				s.counters.errors.Add(1)
				conn.Close()
				select {
				case errs <- err:
				default:
				}
				select {
				case <-time.After(s.cfg.ReconnectDelay):
				case <-ctx.Done():
					return
				}
				continue
			}

			if err := s.consumeStream(ctx, conn, chunks); err != nil {
				conn.Close()
				if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
					select {
					case <-time.After(s.cfg.ReconnectDelay):
					case <-ctx.Done():
						return
					}
					continue
				}
				s.counters.errors.Add(1)
				select {
				case errs <- err:
				default:
				}
				select {
				case <-time.After(s.cfg.ReconnectDelay):
				case <-ctx.Done():
					return
				}
				continue
			}
			conn.Close()
		}
	}()

	return chunks, errs
}

// Metrics returns the RTMP counters.
func (s *RTMPStreamSource) Metrics() StreamMetrics {
	return s.counters.snapshot()
}

func (s *RTMPStreamSource) dial(ctx context.Context) (net.Conn, error) {
	network := "tcp"
	host := s.url.Host
	return s.cfg.Dialer.DialContext(ctx, network, host)
}

func (s *RTMPStreamSource) handshake(conn net.Conn) error {
	if _, err := conn.Write([]byte(handshakeMagic)); err != nil {
		return fmt.Errorf("rtmp handshake send: %w", err)
	}
	buf := make([]byte, len(handshakeMagic))
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("rtmp handshake receive: %w", err)
	}
	if string(buf) != handshakeMagic {
		return fmt.Errorf("unexpected handshake response %q", string(buf))
	}
	return nil
}

func (s *RTMPStreamSource) consumeStream(ctx context.Context, conn net.Conn, chunks chan<- MediaChunk) error {
	header := make([]byte, 4)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.cfg.ReadTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout))
		}
		if _, err := io.ReadFull(conn, header); err != nil {
			return fmt.Errorf("rtmp read header: %w", err)
		}
		length := binary.BigEndian.Uint32(header)
		if length == 0 {
			continue
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return fmt.Errorf("rtmp read payload: %w", err)
		}
		chunk := MediaChunk{
			Sequence:  s.counters.sequence.Add(1),
			Timestamp: time.Now().UTC(),
			Payload:   payload,
			Metadata: map[string]string{
				"path": s.url.Path,
			},
		}
		select {
		case chunks <- chunk:
			s.counters.received.Add(1)
		default:
			s.counters.dropped.Add(1)
		}
	}
}
