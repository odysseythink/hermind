package utils

// AcceptedMIMETypes maps MIME types to their accepted file extensions.
var AcceptedMIMETypes = map[string][]string{
	"text/plain":       {".txt", ".md", ".org", ".adoc", ".rst"},
	"text/html":        {".html"},
	"text/csv":         {".csv"},
	"application/json": {".json"},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   {".docx"},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {".pptx"},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         {".xlsx"},
	"application/vnd.oasis.opendocument.text":                                   {".odt"},
	"application/vnd.oasis.opendocument.presentation":                           {".odp"},
	"application/pdf":      {".pdf"},
	"application/mbox":     {".mbox"},
	"audio/wav":            {".wav"},
	"audio/mpeg":           {".mp3"},
	"audio/ogg":            {".ogg", ".oga"},
	"audio/opus":           {".opus"},
	"audio/mp4":            {".m4a"},
	"audio/x-m4a":          {".m4a"},
	"audio/webm":           {".webm"},
	"video/mp4":            {".mp4"},
	"video/mpeg":           {".mpeg"},
	"application/epub+zip": {".epub"},
	"image/png":            {".png"},
	"image/jpeg":           {".jpg"},
	"image/jpg":            {".jpg"},
	"image/webp":           {".webp"},
}

// AcceptedFileTypes returns a flat slice of all accepted MIME type strings.
func AcceptedFileTypes() []string {
	types := make([]string, 0, len(AcceptedMIMETypes))
	for mime := range AcceptedMIMETypes {
		types = append(types, mime)
	}
	return types
}
