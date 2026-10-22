package previewkit

import (
	"html/template"

	"github.com/raincfhnj/previewkit/internal/markdown"
)

// RenderMarkdown converts a safe CommonMark/GFM subset to an HTML fragment.
// Raw HTML is escaped; link schemes are restricted to http, https, mailto,
// and relative URLs.
func RenderMarkdown(src []byte) (template.HTML, error) {
	out, err := markdown.Render(src)
	if err != nil {
		return "", err
	}
	return template.HTML(out), nil
}
