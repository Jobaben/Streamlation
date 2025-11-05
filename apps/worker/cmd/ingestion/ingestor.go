package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	ingestionpkg "streamlation/packages/backend/ingestion"
	sessionpkg "streamlation/packages/backend/session"

	"go.uber.org/zap"
)

// streamIngestor adapts TranslationSession inputs into ingestion StreamSources.
type streamIngestor struct {
	logger            *zap.SugaredLogger
	httpClient        *http.Client
	dialer            *net.Dialer
	bufferSize        int
	sampleWindow      time.Duration
	fileChunkSize     int
	fileChunkDuration time.Duration
}

func newStreamIngestor(logger *zap.SugaredLogger) *streamIngestor {
	return &streamIngestor{
		logger:            logger,
		httpClient:        &http.Client{Timeout: 10 * time.Second},
		dialer:            &net.Dialer{Timeout: 5 * time.Second},
		bufferSize:        16,
		sampleWindow:      3 * time.Second,
		fileChunkSize:     64 * 1024,
		fileChunkDuration: 200 * time.Millisecond,
	}
}

func (s *streamIngestor) Ingest(ctx context.Context, session sessionpkg.TranslationSession) error {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	source, err := s.buildSource(session)
	if err != nil {
		return err
	}

	chunks, errs := source.Stream(streamCtx)
	timer := time.NewTimer(s.sampleWindow)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			s.logger.Infow("ingestion warmup complete", "sessionID", session.ID, "metrics", source.Metrics())
			return nil
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if err != nil {
				return err
			}
		case chunk, ok := <-chunks:
			if !ok {
				return nil
			}
			s.logger.Debugw("received media chunk", "sessionID", session.ID, "sequence", chunk.Sequence, "duration", chunk.Duration)
		}
	}
}

func (s *streamIngestor) buildSource(session sessionpkg.TranslationSession) (ingestionpkg.StreamSource, error) {
	switch session.Source.Type {
	case "hls":
		return ingestionpkg.NewHLSStreamSource(ingestionpkg.HLSConfig{
			PlaylistURL:  session.Source.URI,
			Client:       s.httpClient,
			BufferSize:   s.bufferSize,
			PollInterval: 1 * time.Second,
		})
	case "rtmp":
		return ingestionpkg.NewRTMPStreamSource(ingestionpkg.RTMPConfig{
			URL:            session.Source.URI,
			Dialer:         s.dialer,
			BufferSize:     s.bufferSize,
			ReconnectDelay: 500 * time.Millisecond,
			ReadTimeout:    3 * time.Second,
		})
	case "file":
		return s.buildFileSource(session)
	case "dash":
		return nil, fmt.Errorf("ingestion adapter for %s not yet implemented", session.Source.Type)
	default:
		return nil, errors.New("unsupported source type")
	}
}

func (s *streamIngestor) buildFileSource(session sessionpkg.TranslationSession) (ingestionpkg.StreamSource, error) {
	uri, err := url.Parse(session.Source.URI)
	if err != nil {
		return nil, fmt.Errorf("invalid file source uri: %w", err)
	}
	if uri.Scheme != "file" {
		return nil, errors.New("file sources must use file:// URIs")
	}

	path := uri.Path
	if uri.Host != "" {
		path = fmt.Sprintf("//%s%s", uri.Host, uri.Path)
	}
	if path == "" {
		return nil, errors.New("file source missing path")
	}

	return ingestionpkg.NewFileStreamSource(ingestionpkg.FileConfig{
		Path:          path,
		ChunkSize:     s.fileChunkSize,
		ChunkDuration: s.fileChunkDuration,
		BufferSize:    s.bufferSize,
		Metadata: map[string]string{
			"source":  "file",
			"session": session.ID,
		},
	})
}

var _ sessionIngestor = (*streamIngestor)(nil)
