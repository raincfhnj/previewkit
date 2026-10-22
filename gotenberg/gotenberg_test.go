package gotenberg

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConvertToPDF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forms/libreoffice/convert" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f, header, err := r.FormFile("files")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()
		if header.Filename != "a.docx" {
			http.Error(w, "wrong filename", http.StatusBadRequest)
			return
		}
		if _, err := io.Copy(io.Discard, f); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4\n"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	pdf, err := c.ConvertToPDF(context.Background(), "a.docx", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if string(pdf) != "%PDF-1.4\n" {
		t.Fatalf("unexpected response: %q", pdf)
	}
}

func TestConvertToPDFError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad document", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.ConvertToPDF(context.Background(), "a.docx", strings.NewReader("hello"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), fmt.Sprint(http.StatusBadRequest)) &&
		!strings.Contains(err.Error(), "400") {
		t.Fatalf("unexpected error: %v", err)
	}
}
