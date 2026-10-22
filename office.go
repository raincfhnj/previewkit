package previewkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cacheFormatVersion is mixed into cache keys. Bump it whenever the converter,
// LibreOffice version, or installed fonts change.
const cacheFormatVersion = "1"

func (e *Engine) renderOffice(ctx context.Context, path string, st os.FileInfo) (*Result, error) {
	if e.converter == nil {
		return nil, ErrConverterRequired
	}

	timeout := e.timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	key := e.cacheKey(path, st)

	if !e.disableCache {
		dst := filepath.Join(e.cacheDir, key+".pdf")
		if pdfStat, err := os.Stat(dst); err == nil && pdfStat.Size() > 0 {
			return officePDFResult(path, dst, pdfStat), nil
		}
		if err := os.MkdirAll(e.cacheDir, 0o755); err != nil {
			return nil, err
		}

		v, err, _ := e.sf.Do(key, func() (any, error) {
			// Another process or goroutine may have populated the cache.
			if pdfStat, err := os.Stat(dst); err == nil && pdfStat.Size() > 0 {
				return dst, nil
			}

			pdf, err := e.convert(ctx, path)
			if err != nil {
				return "", err
			}

			tmp, err := os.CreateTemp(e.cacheDir, key+".*.tmp")
			if err != nil {
				return "", err
			}
			tmpName := tmp.Name()
			if _, err := tmp.Write(pdf); err != nil {
				tmp.Close()
				os.Remove(tmpName)
				return "", err
			}
			if err := tmp.Close(); err != nil {
				os.Remove(tmpName)
				return "", err
			}
			if err := os.Rename(tmpName, dst); err != nil {
				os.Remove(tmpName)
				return "", err
			}
			return dst, nil
		})
		if err != nil {
			return nil, err
		}

		dst = v.(string)
		pdfStat, err := os.Stat(dst)
		if err != nil {
			return nil, err
		}
		return officePDFResult(path, dst, pdfStat), nil
	}

	dir, err := os.MkdirTemp("", "previewkit-*")
	if err != nil {
		return nil, err
	}
	pdf, err := e.convert(ctx, path)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	dst := filepath.Join(dir, key+".pdf")
	if err := os.WriteFile(dst, pdf, 0o644); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	pdfStat, err := os.Stat(dst)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	res := officePDFResult(path, dst, pdfStat)
	res.cleanup = func() error { return os.RemoveAll(dir) }
	return res, nil
}

func (e *Engine) convert(ctx context.Context, path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return e.converter.ConvertToPDF(ctx, filepath.Base(path), f)
}

func (e *Engine) cacheKey(path string, st os.FileInfo) string {
	h := sha256.Sum256([]byte(fmt.Sprintf(
		"%s|%d|%d|%T|%s",
		path,
		st.Size(),
		st.ModTime().UnixNano(),
		e.converter,
		cacheFormatVersion,
	)))
	return hex.EncodeToString(h[:])
}

func officePDFResult(srcPath, pdfPath string, st os.FileInfo) *Result {
	name := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath)) + ".pdf"
	return &Result{
		Kind:        KindOffice,
		ContentType: "application/pdf",
		Filename:    name,
		FilePath:    pdfPath,
		Size:        st.Size(),
		ModTime:     st.ModTime(),
		Converted:   true,
	}
}
