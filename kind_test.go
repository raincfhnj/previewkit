package previewkit

import "testing"

func TestDetectKind(t *testing.T) {
	tests := map[string]Kind{
		"a.md":       KindMarkdown,
		"a.MARKDOWN": KindMarkdown,
		"a.txt":      KindText,
		"a.json":     KindText,
		"a.html":     KindText,
		"a.xlsx":     KindExcel,
		"a.xls":      KindOffice,
		"a.pdf":      KindPDF,
		"a.docx":     KindDocx,
		"a.doc":      KindOffice,
		"a.pptx":     KindOffice,
		"a.png":      KindImage,
		"a.svg":      KindImage,
		"a.mp4":      KindVideo,
		"a.mp3":      KindAudio,
		"a.bin":      KindUnsupported,
	}

	for name, want := range tests {
		if got := DetectKind(name); got != want {
			t.Errorf("DetectKind(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestContentTypeForHTMLIsPlainText(t *testing.T) {
	if got := ContentTypeFor("x.html"); got != "text/plain; charset=utf-8" {
		t.Fatalf("HTML must be previewed as source text, got %q", got)
	}
}
