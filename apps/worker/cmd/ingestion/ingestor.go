package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	ingestionpkg "streamlation/packages/backend/ingestion"
	sessionpkg "streamlation/packages/backend/session"

	"go.uber.org/zap"
)

// streamIngestor adapts TranslationSession inputs into ingestion StreamSources.
type streamIngestor struct {
	logger       *zap.SugaredLogger
	httpClient   *http.Client
	dialer       *net.Dialer
	bufferSize   int
	sampleWindow time.Duration
}

func newStreamIngestor(logger *zap.SugaredLogger) *streamIngestor {
	return &streamIngestor{
		logger:       logger,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		dialer:       &net.Dialer{Timeout: 5 * time.Second},
		bufferSize:   16,
		sampleWindow: 3 * time.Second,
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
	case "dash", "file":
		return nil, fmt.Errorf("ingestion adapter for %s not yet implemented", session.Source.Type)
	default:
		return nil, errors.New("unsupported source type")
	}
}

var _ sessionIngestor = (*streamIngestor)(nil)
