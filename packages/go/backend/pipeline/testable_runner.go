package pipeline

import (
	"bytes"
	"context"
	"io"
	"time"

	"streamlation/packages/backend/asr"
	"streamlation/packages/backend/media"
	"streamlation/packages/backend/output"
	sessionpkg "streamlation/packages/backend/session"
	statuspkg "streamlation/packages/backend/status"
	"streamlation/packages/backend/translation"
)

// TestableRunner wires stub components together to produce realistic data flow
// through the full pipeline without requiring real AI models or FFmpeg.
type TestableRunner struct {
	normalizer media.Normalizer
	recognizer asr.Recognizer
	translator translation.Translator
	generator  output.SubtitleGenerator
}

// NewTestableRunner creates a testable pipeline runner with the given components.
func NewTestableRunner(
	normalizer media.Normalizer,
	recognizer asr.Recognizer,
	translator translation.Translator,
	generator output.SubtitleGenerator,
) *TestableRunner {
	return &TestableRunner{
		normalizer: normalizer,
		recognizer: recognizer,
		translator: translator,
		generator:  generator,
	}
}

// Run executes the full pipeline for a session using the wired stub components.
// It emits status events at each stage transition and produces real subtitle output.
func (r *TestableRunner) Run(ctx context.Context, session sessionpkg.TranslationSession, emit func(statuspkg.SessionStatusEvent) error) error {
	if emit == nil {
		emit = func(statuspkg.SessionStatusEvent) error { return nil }
	}

	// Stage 1: Ingestion (simulated with empty reader)
	if err := r.emitStatus(emit, session.ID, "ingestion", "running", "Starting stream ingestion"); err != nil {
		return err
	}

	// Create a simulated media source
	source := bytes.NewReader([]byte{})

	if err := r.emitStatus(emit, session.ID, "ingestion", "completed", "Stream ingested successfully"); err != nil {
		return err
	}

	// Stage 2: Normalization
	if err := r.emitStatus(emit, session.ID, "normalization", "running", "Normalizing audio"); err != nil {
		return err
	}

	chunks, err := r.normalizer.Normalize(ctx, source)
	if err != nil {
		return r.emitStatus(emit, session.ID, "normalization", "failed", err.Error())
	}

	if err := r.emitStatus(emit, session.ID, "normalization", "completed", "Audio normalized"); err != nil {
		return err
	}

	// Stage 3: ASR
	if err := r.emitStatus(emit, session.ID, "asr", "running", "Transcribing audio"); err != nil {
		return err
	}

	transcripts, err := r.recognizer.Recognize(ctx, session.ID, chunks)
	if err != nil {
		return r.emitStatus(emit, session.ID, "asr", "failed", err.Error())
	}

	if err := r.emitStatus(emit, session.ID, "asr", "completed", "Audio transcribed"); err != nil {
		return err
	}

	// Stage 4: Translation
	if err := r.emitStatus(emit, session.ID, "translation", "running", "Translating to "+session.TargetLanguage); err != nil {
		return err
	}

	translations, err := r.translator.TranslateStream(ctx, session.ID, transcripts, session.TargetLanguage)
	if err != nil {
		return r.emitStatus(emit, session.ID, "translation", "failed", err.Error())
	}

	if err := r.emitStatus(emit, session.ID, "translation", "completed", "Translation complete"); err != nil {
		return err
	}

	// Stage 5: Output Generation
	if err := r.emitStatus(emit, session.ID, "output", "running", "Generating subtitles"); err != nil {
		return err
	}

	// Stream subtitle events
	events, err := r.generator.StreamSubtitles(ctx, session.ID, translations)
	if err != nil {
		return r.emitStatus(emit, session.ID, "output", "failed", err.Error())
	}

	// Consume all subtitle events
	subtitleCount := 0
	for range events {
		subtitleCount++
	}

	if err := r.emitStatus(emit, session.ID, "output", "completed",
		"Generated "+itoa(subtitleCount)+" subtitles"); err != nil {
		return err
	}

	return nil
}

// emitStatus sends a status event through the emit function.
func (r *TestableRunner) emitStatus(emit func(statuspkg.SessionStatusEvent) error, sessionID, stage, state, detail string) error {
	return emit(statuspkg.SessionStatusEvent{
		SessionID: sessionID,
		Stage:     stage,
		State:     state,
		Detail:    detail,
		Timestamp: time.Now().UTC(),
	})
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// RunWithReader executes the pipeline with a provided media source reader.
// This is useful for testing with actual media data.
func (r *TestableRunner) RunWithReader(ctx context.Context, session sessionpkg.TranslationSession, source io.Reader, emit func(statuspkg.SessionStatusEvent) error) error {
	if emit == nil {
		emit = func(statuspkg.SessionStatusEvent) error { return nil }
	}

	// Stage 1: Ingestion
	if err := r.emitStatus(emit, session.ID, "ingestion", "running", "Starting stream ingestion"); err != nil {
		return err
	}

	if err := r.emitStatus(emit, session.ID, "ingestion", "completed", "Stream ingested"); err != nil {
		return err
	}

	// Stage 2: Normalization
	if err := r.emitStatus(emit, session.ID, "normalization", "running", "Normalizing audio"); err != nil {
		return err
	}

	chunks, err := r.normalizer.Normalize(ctx, source)
	if err != nil {
		return r.emitStatus(emit, session.ID, "normalization", "failed", err.Error())
	}

	if err := r.emitStatus(emit, session.ID, "normalization", "completed", "Audio normalized"); err != nil {
		return err
	}

	// Stage 3: ASR
	if err := r.emitStatus(emit, session.ID, "asr", "running", "Transcribing audio"); err != nil {
		return err
	}

	transcripts, err := r.recognizer.Recognize(ctx, session.ID, chunks)
	if err != nil {
		return r.emitStatus(emit, session.ID, "asr", "failed", err.Error())
	}

	if err := r.emitStatus(emit, session.ID, "asr", "completed", "Audio transcribed"); err != nil {
		return err
	}

	// Stage 4: Translation
	if err := r.emitStatus(emit, session.ID, "translation", "running", "Translating to "+session.TargetLanguage); err != nil {
		return err
	}

	translations, err := r.translator.TranslateStream(ctx, session.ID, transcripts, session.TargetLanguage)
	if err != nil {
		return r.emitStatus(emit, session.ID, "translation", "failed", err.Error())
	}

	if err := r.emitStatus(emit, session.ID, "translation", "completed", "Translation complete"); err != nil {
		return err
	}

	// Stage 5: Output Generation
	if err := r.emitStatus(emit, session.ID, "output", "running", "Generating subtitles"); err != nil {
		return err
	}

	events, err := r.generator.StreamSubtitles(ctx, session.ID, translations)
	if err != nil {
		return r.emitStatus(emit, session.ID, "output", "failed", err.Error())
	}

	subtitleCount := 0
	for range events {
		subtitleCount++
	}

	if err := r.emitStatus(emit, session.ID, "output", "completed",
		"Generated "+itoa(subtitleCount)+" subtitles"); err != nil {
		return err
	}

	return nil
}
