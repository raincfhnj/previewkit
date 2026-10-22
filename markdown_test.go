package previewkit

import (
	"strings"
	"testing"
)

func TestRenderMarkdownSanitizesScript(t *testing.T) {
	src := []byte("# Hello\n\n<script>alert(1)</script>\n\n| a | b |\n|---|---|\n| 1 | 2 |\n")
	out, err := RenderMarkdown(src)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(strings.ToLower(s), "<script") {
		t.Fatalf("script tag survived sanitization: %s", s)
	}
	if !strings.Contains(s, "<h1") {
		t.Fatalf("heading was not rendered: %s", s)
	}
	if !strings.Contains(s, "<table>") {
		t.Fatalf("GFM table was not rendered: %s", s)
	}
}
