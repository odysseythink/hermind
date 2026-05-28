package core

import "errors"

var (
	ErrUnsupportedFormat   = errors.New("unsupported file format")
	ErrFileNotFound        = errors.New("file not found")
	ErrInvalidPath         = errors.New("invalid file path")
	ErrEmptyContent        = errors.New("no text content found")
	ErrProcessingTimeout   = errors.New("processing timeout")
	ErrOCRFailed           = errors.New("OCR processing failed")
	ErrTranscriptionFailed = errors.New("audio transcription failed")
	ErrBrowserLaunchFailed = errors.New("browser launch failed")
)
