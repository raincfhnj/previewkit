package previewkit

import (
	"html/template"
	"time"
)

// Result is the browser-previewable representation of a file.
//
// Exactly one payload field is populated:
//
//   - HTML for KindMarkdown and KindExcel. The fragment is already sanitized
//     for Markdown and escaped for spreadsheets.
//   - Text for KindText.
//   - FilePath for PDF, images, audio, video, and converted Office documents.
//
// Callers serving FilePath should use http.ServeContent (or equivalent) so
// Range requests work for PDF pages and media seeking.
type Result struct {
	Kind        Kind
	ContentType string
	Filename    string

	// HTML is a sanitized HTML fragment.
	HTML template.HTML

	// Text is decoded UTF-8 text.
	Text string

	// FilePath is an absolute path to the original file or to a converted
	// PDF. It is empty for HTML and text results.
	FilePath string

	Size    int64
	ModTime time.Time

	// Converted reports whether FilePath points to a generated PDF rather
	// than the original file.
	Converted bool

	cleanup func() error
}

// IsHTML reports whether the result contains an HTML fragment.
func (r *Result) IsHTML() bool {
	return r != nil && (r.Kind == KindMarkdown || r.Kind == KindExcel || r.Kind == KindDocx)
}

// IsText reports whether the result contains decoded plain text.
func (r *Result) IsText() bool {
	return r != nil && r.Kind == KindText
}

// IsFile reports whether the result points to a file on disk.
func (r *Result) IsFile() bool {
	return r != nil && r.FilePath != ""
}

// Close releases temporary files created when the engine runs without a
// cache. It is safe to call multiple times. Cached files are not removed.
func (r *Result) Close() error {
	if r == nil || r.cleanup == nil {
		return nil
	}
	err := r.cleanup()
	r.cleanup = nil
	return err
}
