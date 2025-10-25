package session

// TranslationSession models the configuration for a translation session.
type TranslationSession struct {
	ID             string             `json:"id"`
	Source         TranslationSource  `json:"source"`
	TargetLanguage string             `json:"targetLanguage"`
	Options        TranslationOptions `json:"options"`
}

// TranslationSource describes the input stream configuration.
type TranslationSource struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
}

// TranslationOptions contains tuning values for a session.
type TranslationOptions struct {
	EnableDubbing      bool   `json:"enableDubbing"`
	LatencyToleranceMs int    `json:"latencyToleranceMs"`
	ModelProfile       string `json:"modelProfile"`
}
