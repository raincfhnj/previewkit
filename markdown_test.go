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

func TestRenderMarkdownInline(t *testing.T) {
	src := []byte("**bold** *em* __strong__ ~~del~~ `code` [link](https://example.com)\n\n<img src=x onerror=alert(1)>\n\n[bad](javascript:alert(1))")
	out, err := RenderMarkdown(src)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)

	for _, want := range []string{
		"<strong>bold</strong>",
		"<em>em</em>",
		"<strong>strong</strong>",
		"<del>del</del>",
		"<code>code</code>",
		`<a href="https://example.com"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if strings.Contains(strings.ToLower(s), `href="javascript:`) {
		t.Fatalf("javascript URL was not rejected: %s", s)
	}
	if strings.Contains(strings.ToLower(s), "<img src=x") {
		t.Fatalf("raw HTML was not escaped: %s", s)
	}
}
