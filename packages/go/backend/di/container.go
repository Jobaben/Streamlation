package di

import (
	"streamlation/packages/backend/asr"
	"streamlation/packages/backend/media"
	"streamlation/packages/backend/output"
	"streamlation/packages/backend/pipeline"
	"streamlation/packages/backend/translation"
	"streamlation/packages/backend/tts"
)

// Container holds all service dependencies for the translation pipeline.
// It enables dependency injection for both production and test environments.
type Container struct {
	Normalizer  media.Normalizer
	Recognizer  asr.Recognizer
	Translator  translation.Translator
	Synthesizer tts.Synthesizer
	Generator   output.SubtitleGenerator
	Runner      pipeline.Runner
}

// ContainerOption configures a container during construction.
type ContainerOption func(*Container)

// WithNormalizer sets the normalizer implementation.
func WithNormalizer(n media.Normalizer) ContainerOption {
	return func(c *Container) { c.Normalizer = n }
}

// WithRecognizer sets the ASR recognizer implementation.
func WithRecognizer(r asr.Recognizer) ContainerOption {
	return func(c *Container) { c.Recognizer = r }
}

// WithTranslator sets the translator implementation.
func WithTranslator(t translation.Translator) ContainerOption {
	return func(c *Container) { c.Translator = t }
}

// WithSynthesizer sets the TTS synthesizer implementation.
func WithSynthesizer(s tts.Synthesizer) ContainerOption {
	return func(c *Container) { c.Synthesizer = s }
}

// WithGenerator sets the subtitle generator implementation.
func WithGenerator(g output.SubtitleGenerator) ContainerOption {
	return func(c *Container) { c.Generator = g }
}

// WithRunner sets the pipeline runner implementation.
func WithRunner(r pipeline.Runner) ContainerOption {
	return func(c *Container) { c.Runner = r }
}

// NewTestContainer creates a container with all stub implementations
// for testing without external dependencies.
func NewTestContainer() *Container {
	normalizer := media.NewStubNormalizer(nil)
	recognizer := asr.NewStubRecognizer(nil)
	translator := translation.NewStubTranslator(nil)
	synthesizer := tts.NewStubSynthesizer(nil)
	generator := output.NewStubGenerator()

	c := &Container{
		Normalizer:  normalizer,
		Recognizer:  recognizer,
		Translator:  translator,
		Synthesizer: synthesizer,
		Generator:   generator,
	}

	// Create testable runner wired with stub components
	c.Runner = pipeline.NewTestableRunner(normalizer, recognizer, translator, generator)

	return c
}

// NewContainer creates a container with the given options.
func NewContainer(opts ...ContainerOption) *Container {
	c := &Container{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
