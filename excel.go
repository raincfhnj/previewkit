package previewkit

import (
	"html/template"

	"github.com/raincfhnj/previewkit/internal/xlsx"
)

// ExcelOptions controls spreadsheet rendering.
type ExcelOptions struct {
	// MaxRows is the maximum number of rows rendered per sheet.
	// Values <= 0 use 2000.
	MaxRows int

	// MaxCols is the maximum number of columns rendered per sheet.
	// Values <= 0 use 100.
	MaxCols int

	// MaxXMLBytes limits each uncompressed OOXML part. Values <= 0 use 32 MiB.
	MaxXMLBytes int64

	// MaxImageBytes limits one embedded image. Values <= 0 use 4 MiB.
	MaxImageBytes int64

	// MaxTotalImageBytes bounds all embedded images in one workbook.
	// Values <= 0 use 20 MiB.
	MaxTotalImageBytes int64
}

// RenderXLSX converts an XLSX/XLSM workbook to an HTML fragment.
//
// The fragment contains pk-workbook, pk-tabs, pk-tab, pk-sheet, pk-table, and
// pk-cell classes. It preserves values, merged cells, column widths, row
// heights, and a subset of cell styling.
func RenderXLSX(path string, opts ExcelOptions) (template.HTML, error) {
	return xlsx.Render(path, xlsx.Options{
		MaxRows:            opts.MaxRows,
		MaxCols:            opts.MaxCols,
		MaxXMLBytes:        opts.MaxXMLBytes,
		MaxImageBytes:      opts.MaxImageBytes,
		MaxTotalImageBytes: opts.MaxTotalImageBytes,
	})
}
