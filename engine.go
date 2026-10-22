package previewkit

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Converter converts an Office document to PDF.
//
// Implementations must be safe for concurrent use. The reader is the original
// document; filename is only a hint used to preserve the extension and to
// derive the output name.
type Converter interface {
	ConvertToPDF(ctx context.Context, filename string, r io.Reader) ([]byte, error)
}

// Engine renders files with a fixed configuration. It is safe for concurrent
// use after construction.
type Engine struct {
	converter    Converter
	cacheDir     string
	disableCache bool
	maxTextBytes int64
	maxExcelRows int
	maxExcelCols int
	maxXMLBytes  int64
	timeout      time.Duration

	sf group
}

// Option configures an Engine.
type Option func(*Engine)

// WithConverter enables Office previews through c.
func WithConverter(c Converter) Option {
	return func(e *Engine) { e.converter = c }
}

// WithCacheDir sets the directory used for converted PDFs. The directory is
// created lazily. An empty directory is ignored.
func WithCacheDir(dir string) Option {
	return func(e *Engine) {
		if dir != "" {
			e.cacheDir = dir
		}
	}
}

// WithoutCache writes converted PDFs to a temporary directory instead of the
// persistent cache. Results then own a cleanup function; callers should call
// Result.Close after serving the file.
func WithoutCache() Option {
	return func(e *Engine) { e.disableCache = true }
}

// WithTimeout bounds a single Office conversion. The default is 90 seconds.
func WithTimeout(d time.Duration) Option {
	return func(e *Engine) {
		if d > 0 {
			e.timeout = d
		}
	}
}

// WithMaxTextBytes limits Markdown and text files rendered inline. Zero means
// no limit. The default is 8 MiB.
func WithMaxTextBytes(n int64) Option {
	return func(e *Engine) { e.maxTextBytes = n }
}

// WithMaxXMLBytes limits each uncompressed OOXML part in XLSX/DOCX
// documents. The default is 32 MiB.
func WithMaxXMLBytes(n int64) Option {
	return func(e *Engine) {
		if n > 0 {
			e.maxXMLBytes = n
		}
	}
}

// WithMaxExcelRows limits the number of rows rendered per worksheet.
// The default is 2000.
func WithMaxExcelRows(n int) Option {
	return func(e *Engine) {
		if n > 0 {
			e.maxExcelRows = n
		}
	}
}

// WithMaxExcelCols limits the number of columns rendered per worksheet.
// The default is 100.
func WithMaxExcelCols(n int) Option {
	return func(e *Engine) {
		if n > 0 {
			e.maxExcelCols = n
		}
	}
}

// New creates an Engine.
func New(opts ...Option) *Engine {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		cacheRoot = os.TempDir()
	}

	e := &Engine{
		cacheDir:     filepath.Join(cacheRoot, "github.com/raincfhnj/previewkit"),
		maxTextBytes: 8 << 20,
		maxExcelRows: 2000,
		maxExcelCols: 100,
		maxXMLBytes:  32 << 20,
		timeout:      90 * time.Second,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Render inspects path and returns a browser-previewable representation.
//
// Markdown, XLSX, DOCX, and PPTX are rendered in-process. Legacy Office
// formats require a Converter. PDF, image, audio, and video results are
// returned as file paths; the caller is responsible for serving them with
// Range support and a safe Content-Type.
func (e *Engine) Render(ctx context.Context, path string) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s", ErrNotRegularFile, abs)
	}

	base := filepath.Base(abs)
	kind := DetectKind(abs)

	switch kind {
	case KindMarkdown:
		src, err := readFileLimited(abs, e.maxTextBytes)
		if err != nil {
			return nil, err
		}
		html, err := RenderMarkdown(src)
		if err != nil {
			return nil, err
		}
		return &Result{
			Kind:        KindMarkdown,
			ContentType: "text/html; charset=utf-8",
			Filename:    base,
			HTML:        html,
			Size:        st.Size(),
			ModTime:     st.ModTime(),
		}, nil

	case KindText:
		src, err := readFileLimited(abs, e.maxTextBytes)
		if err != nil {
			return nil, err
		}
		if isBinary(src) {
			return nil, fmt.Errorf("%w: %s", ErrBinary, base)
		}
		return &Result{
			Kind:        KindText,
			ContentType: ContentTypeFor(base),
			Filename:    base,
			Text:        DecodeText(src),
			Size:        st.Size(),
			ModTime:     st.ModTime(),
		}, nil

	case KindExcel:
		html, err := RenderXLSX(abs, ExcelOptions{
			MaxRows:     e.maxExcelRows,
			MaxCols:     e.maxExcelCols,
			MaxXMLBytes: e.maxXMLBytes,
		})
		if err != nil {
			return nil, fmt.Errorf("previewkit: render xlsx: %w", err)
		}
		return &Result{
			Kind:        KindExcel,
			ContentType: "text/html; charset=utf-8",
			Filename:    base,
			HTML:        html,
			Size:        st.Size(),
			ModTime:     st.ModTime(),
		}, nil

	case KindDocx:
		html, err := RenderDOCX(abs, DocxOptions{MaxXMLBytes: e.maxXMLBytes})
		if err != nil {
			return nil, fmt.Errorf("previewkit: render docx: %w", err)
		}
		return &Result{
			Kind:        KindDocx,
			ContentType: "text/html; charset=utf-8",
			Filename:    base,
			HTML:        html,
			Size:        st.Size(),
			ModTime:     st.ModTime(),
		}, nil

	case KindPptx:
		html, err := RenderPPTX(abs, PptxOptions{MaxXMLBytes: e.maxXMLBytes})
		if err != nil {
			return nil, fmt.Errorf("previewkit: render pptx: %w", err)
		}
		return &Result{
			Kind:        KindPptx,
			ContentType: "text/html; charset=utf-8",
			Filename:    base,
			HTML:        html,
			Size:        st.Size(),
			ModTime:     st.ModTime(),
		}, nil

	case KindOffice:
		return e.renderOffice(ctx, abs, st)

	case KindPDF, KindImage, KindVideo, KindAudio:
		return &Result{
			Kind:        kind,
			ContentType: ContentTypeFor(base),
			Filename:    base,
			FilePath:    abs,
			Size:        st.Size(),
			ModTime:     st.ModTime(),
		}, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, base)
	}
}

func readFileLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if limit <= 0 {
		return io.ReadAll(f)
	}

	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%w: limit is %d bytes", ErrTooLarge, limit)
	}
	return b, nil
}
