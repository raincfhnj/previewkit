package previewkit

import (
	"bytes"
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Footnote,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
)

var markdownPolicy = bluemonday.UGCPolicy()

// RenderMarkdown converts CommonMark/GFM to a sanitized HTML fragment.
//
// Raw HTML in the source is sanitized with bluemonday's UGC policy. The
// returned template.HTML is safe to embed in a page.
func RenderMarkdown(src []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := markdownRenderer.Convert(src, &buf); err != nil {
		return "", err
	}
	safe := markdownPolicy.SanitizeBytes(buf.Bytes())
	return template.HTML(safe), nil
}
