package core

// Options represents the collector processing options sent with each request.
type Options struct {
	WhisperProvider  string          `json:"whisperProvider"`
	WhisperModelPref string          `json:"WhisperModelPref,omitempty"`
	OpenAiKey        string          `json:"openAiKey,omitempty"`
	OCR              OCROptions      `json:"ocr"`
	RuntimeSettings  RuntimeSettings `json:"runtimeSettings"`
}

// OCROptions holds OCR-specific configuration.
type OCROptions struct {
	LangList string `json:"langList"`
}

// RuntimeSettings holds collector runtime configuration persisted across requests.
type RuntimeSettings struct {
	AllowAnyIp        string   `json:"allowAnyIp"`
	BrowserLaunchArgs []string `json:"browserLaunchArgs"`
}
