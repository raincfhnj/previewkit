// Package libreoffice implements previewkit.Converter with a local
// LibreOffice installation.
//
// It invokes the soffice binary in headless mode, using a fresh temporary
// profile for each conversion so concurrent calls do not share LibreOffice
// lock files.
//
// LibreOffice is licensed primarily under MPL-2.0/LGPL-3.0. It is an optional
// external program, not a dependency of the previewkit module.
package libreoffice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Converter runs a local LibreOffice binary.
type Converter struct {
	// Binary is the soffice executable. Empty means "soffice" on PATH.
	Binary string

	// Timeout bounds one conversion. Zero means 90 seconds.
	Timeout time.Duration
}

// New returns a Converter. An empty binary means "soffice".
func New(binary string) *Converter {
	if binary == "" {
		binary = "soffice"
	}
	return &Converter{Binary: binary, Timeout: 90 * time.Second}
}

// ConvertToPDF implements previewkit.Converter.
func (c *Converter) ConvertToPDF(ctx context.Context, filename string, r io.Reader) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil {
		return nil, errors.New("libreoffice: nil converter")
	}

	bin := c.Binary
	if bin == "" {
		bin = "soffice"
	}
	bin, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("libreoffice: executable not found: %w", err)
	}

	dir, err := os.MkdirTemp("", "previewkit-lo-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	outDir := filepath.Join(dir, "out")
	profileDir := filepath.Join(dir, "profile")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" || len(ext) > 16 {
		ext = ".bin"
	}
	inputPath := filepath.Join(dir, "input"+ext)

	in, err := os.Create(inputPath)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(in, r); err != nil {
		in.Close()
		return nil, err
	}
	if err := in.Close(); err != nil {
		return nil, err
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin,
		"--headless",
		"--norestore",
		"--nolockcheck",
		"--nodefault",
		"--nofirststartwizard",
		"-env:UserInstallation="+fileURL(profileDir),
		"--convert-to", "pdf",
		"--outdir", outDir,
		inputPath,
	)
	// Give LibreOffice a short grace period to exit after context cancellation.
	cmd.WaitDelay = 5 * time.Second

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("libreoffice: %w: %s", err, strings.TrimSpace(string(output)))
	}

	pdfPath := filepath.Join(outDir, "input.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		matches, _ := filepath.Glob(filepath.Join(outDir, "*.pdf"))
		if len(matches) == 0 {
			return nil, fmt.Errorf("libreoffice: no PDF produced: %s", strings.TrimSpace(string(output)))
		}
		pdfPath = matches[0]
	}

	return os.ReadFile(pdfPath)
}

func fileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}
