package collector

import "github.com/odysseythink/hermind/backend/internal/collector/core"

// Options represents the collector processing options sent with each request.
type Options = core.Options

// OCROptions holds OCR-specific configuration.
type OCROptions = core.OCROptions

// RuntimeSettings holds collector runtime configuration persisted across requests.
type RuntimeSettings = core.RuntimeSettings
