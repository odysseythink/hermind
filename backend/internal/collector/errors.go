package collector

import "github.com/odysseythink/hermind/backend/internal/collector/core"

var (
	ErrUnsupportedFormat   = core.ErrUnsupportedFormat
	ErrFileNotFound        = core.ErrFileNotFound
	ErrInvalidPath         = core.ErrInvalidPath
	ErrEmptyContent        = core.ErrEmptyContent
	ErrProcessingTimeout   = core.ErrProcessingTimeout
	ErrOCRFailed           = core.ErrOCRFailed
	ErrTranscriptionFailed = core.ErrTranscriptionFailed
	ErrBrowserLaunchFailed = core.ErrBrowserLaunchFailed
)
