package collector

import "github.com/odysseythink/hermind/backend/internal/collector/core"

// ProcessResponse is the standard response from document processing endpoints.
type ProcessResponse = core.ProcessResponse

// Document represents a single processed document.
type Document = core.Document

// LinkContentResponse is the response from the get-link utility endpoint.
type LinkContentResponse = core.LinkContentResponse

// ExtensionResponse is the response from collector extension endpoints.
type ExtensionResponse = core.ExtensionResponse

// ParseOptions provides additional options for the parse endpoint.
type ParseOptions = core.ParseOptions
