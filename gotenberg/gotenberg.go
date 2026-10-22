// Package gotenberg implements previewkit.Converter with a Gotenberg service.
//
// Start a LibreOffice-capable instance:
//
//	docker run --rm -p 3000:3000 gotenberg/gotenberg:8-libreoffice
//
// Then configure previewkit:
//
//	engine := previewkit.New(
//	    previewkit.WithConverter(gotenberg.New("http://127.0.0.1:3000")),
//	)
//
// Gotenberg is MIT licensed. Its conversion engine is LibreOffice.
package gotenberg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Converter sends documents to a Gotenberg /forms/libreoffice/convert
// endpoint.
type Converter struct {
	Endpoint string
	Client   *http.Client
}

// New returns a Converter for endpoint. The endpoint should not include a
// trailing slash.
func New(endpoint string) *Converter {
	return &Converter{
		Endpoint: strings.TrimRight(endpoint, "/"),
		Client:   &http.Client{Timeout: 90 * time.Second},
	}
}

// ConvertToPDF implements previewkit.Converter.
func (c *Converter) ConvertToPDF(ctx context.Context, filename string, r io.Reader) ([]byte, error) {
	if c == nil || c.Endpoint == "" {
		return nil, fmt.Errorf("gotenberg: empty endpoint")
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("files", filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, r); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.Endpoint+"/forms/libreoffice/convert", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("gotenberg: %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return io.ReadAll(resp.Body)
}
