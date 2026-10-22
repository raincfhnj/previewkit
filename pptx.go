package previewkit

import (
	"html/template"

	"github.com/raincfhnj/previewkit/internal/pptx"
)

// PptxOptions controls PPTX rendering.
type PptxOptions struct {
	// TargetWidth is the rendered slide width in CSS pixels.
	// Values <= 0 use 960.
	TargetWidth int

	// MaxXMLBytes limits each uncompressed OOXML part.
	// Values <= 0 use 32 MiB.
	MaxXMLBytes int64

	// MaxImageBytes limits one embedded image. Values <= 0 use 4 MiB.
	MaxImageBytes int

	// MaxTotalImageBytes bounds all embedded images in one presentation.
	// Values <= 0 use 20 MiB.
	MaxTotalImageBytes int
}

// RenderPPTX converts a PPTX presentation to an HTML fragment.
//
// This is a content preview: text boxes, basic shapes, tables, pictures, and
// slide backgrounds are supported. Charts, SmartArt, animations, transitions,
// and exact theme/master inheritance are not.
func RenderPPTX(path string, opts ...PptxOptions) (template.HTML, error) {
	var o PptxOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	return pptx.Render(path, pptx.Options{
		TargetWidth:        o.TargetWidth,
		MaxXMLBytes:        o.MaxXMLBytes,
		MaxImageBytes:      o.MaxImageBytes,
		MaxTotalImageBytes: o.MaxTotalImageBytes,
	})
}
