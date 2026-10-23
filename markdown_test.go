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

func TestRenderMarkdownNestedList(t *testing.T) {
	src := []byte("- a\n  - b\n  - c\n- d\n")
	out, err := RenderMarkdown(src)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Count(s, "<ul>") != 2 {
		t.Fatalf("expected two nested lists, got %s", s)
	}
	for _, want := range []string{"<li>a", "<li>b</li>", "<li>c</li>", "<li>d</li>"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
}

func TestRenderMarkdownReferenceLinks(t *testing.T) {
	src := []byte("[foo][id] and ![pic][img] and [shortcut]\n\n" +
		"[id]: https://example.com \"Title\"\n" +
		"[img]: https://example.com/a.png\n" +
		"[shortcut]: https://example.com/short\n" +
		"[unused]: https://example.com/unused\n")
	out, err := RenderMarkdown(src)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		`<a href="https://example.com" title="Title"`,
		`<img src="https://example.com/a.png" alt="pic"`,
		`<a href="https://example.com/short"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if strings.Contains(s, "[id]:") {
		t.Fatalf("reference definition leaked into output: %s", s)
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
