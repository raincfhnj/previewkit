package previewkit

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type fakeConverter struct {
	calls int
}

func (f *fakeConverter) ConvertToPDF(_ context.Context, _ string, _ io.Reader) ([]byte, error) {
	f.calls++
	return []byte("%PDF-1.4\nfake"), nil
}

func TestRenderOfficeWithConverterAndCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.doc")
	if err := os.WriteFile(path, []byte("fake doc"), 0o644); err != nil {
		t.Fatal(err)
	}

	conv := &fakeConverter{}
	engine := New(
		WithConverter(conv),
		WithCacheDir(filepath.Join(dir, "cache")),
	)

	res, err := engine.Render(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Close()

	if res.Kind != KindOffice || !res.Converted {
		t.Fatalf("unexpected result: kind=%q converted=%v", res.Kind, res.Converted)
	}
	if res.ContentType != "application/pdf" {
		t.Fatalf("unexpected content type: %q", res.ContentType)
	}
	if res.Filename != "a.pdf" {
		t.Fatalf("unexpected filename: %q", res.Filename)
	}
	if _, err := os.Stat(res.FilePath); err != nil {
		t.Fatal(err)
	}

	// A second render should hit the disk cache, not the converter.
	if _, err := engine.Render(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if conv.calls != 1 {
		t.Fatalf("converter called %d times, want 1", conv.calls)
	}
}

func TestWithoutCacheCleansUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.doc")
	if err := os.WriteFile(path, []byte("fake doc"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := New(
		WithConverter(&fakeConverter{}),
		WithoutCache(),
	)
	res, err := engine.Render(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	filePath := res.FilePath

	if err := res.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary PDF still exists: %v", err)
	}
}
