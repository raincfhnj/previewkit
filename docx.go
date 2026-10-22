package previewkit

import (
	"html/template"

	"github.com/raincfhnj/previewkit/internal/docx"
)

// DocxOptions controls DOCX rendering limits.
type DocxOptions struct {
	// MaxImageBytes is the largest single embedded image that will be
	// inlined as a data URI. Values <= 0 use 4 MiB.
	MaxImageBytes int

	// MaxTotalImageBytes bounds all embedded images in one document.
	// Values <= 0 use 20 MiB.
	MaxTotalImageBytes int

	// MaxXMLBytes limits each uncompressed OOXML part. Values <= 0 use 32 MiB.
	MaxXMLBytes int64
}

// RenderDOCX converts a DOCX document to an HTML fragment.
//
// This is a content preview: paragraphs, headings, tables, basic character
// formatting, lists, hyperlinks, and inline images are supported. It is not a
// page-perfect Word layout engine.
func RenderDOCX(path string, opts ...DocxOptions) (template.HTML, error) {
	var o DocxOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	return docx.Render(path, docx.Options{
		MaxImageBytes:      o.MaxImageBytes,
		MaxTotalImageBytes: o.MaxTotalImageBytes,
		MaxXMLBytes:        o.MaxXMLBytes,
	})
}
