package previewkit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderMarkdownFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.md")
	if err := os.WriteFile(path, []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := New().Render(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsHTML() {
		t.Fatalf("expected HTML result, got %q", res.Kind)
	}
	if !strings.Contains(string(res.HTML), "<h1") {
		t.Fatalf("unexpected HTML: %s", res.HTML)
	}
	if res.ContentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", res.ContentType)
	}
}

func TestRenderPDFPassthrough(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := New().Render(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsFile() {
		t.Fatal("expected a file result")
	}
	if res.Kind != KindPDF {
		t.Fatalf("unexpected kind: %q", res.Kind)
	}
	if res.ContentType != "application/pdf" {
		t.Fatalf("unexpected content type: %q", res.ContentType)
	}
}

func TestRenderOfficeRequiresConverter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.doc")
	if err := os.WriteFile(path, []byte("not a real doc"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := New().Render(context.Background(), path)
	if !errors.Is(err, ErrConverterRequired) {
		t.Fatalf("expected ErrConverterRequired, got %v", err)
	}
}

func TestRenderUnsupported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(path, []byte{0x00, 0x01}, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := New().Render(context.Background(), path)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}
