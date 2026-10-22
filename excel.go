package previewkit

import (
	"errors"
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExcelOptions controls spreadsheet rendering.
type ExcelOptions struct {
	// MaxRows is the maximum number of rows rendered per sheet.
	// Values <= 0 use 5000.
	MaxRows int

	// MaxCols is the maximum number of columns rendered per sheet.
	// Values <= 0 use 200.
	MaxCols int
}

type cellSpan struct {
	rowspan int
	colspan int
}

// RenderXLSX converts an XLSX/XLSM workbook to an HTML fragment.
//
// The fragment contains pk-workbook, pk-tabs, pk-tab, pk-sheet, pk-table, and
// pk-cell classes. It preserves values, merged cells, column widths, row
// heights, and a subset of cell styling (bold, italic, underline, font size,
// font color, fill color, alignment, and wrapping).
//
// Charts, images, conditional formatting, pivot tables, and formulas without
// cached values are not rendered as Excel would render them.
func RenderXLSX(path string, opts ExcelOptions) (template.HTML, error) {
	if opts.MaxRows <= 0 {
		opts.MaxRows = 5000
	}
	if opts.MaxCols <= 0 {
		opts.MaxCols = 200
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return "", errors.New("previewkit: workbook has no sheets")
	}

	var b strings.Builder
	b.WriteString(`<div class="pk-workbook">`)
	b.WriteString(`<div class="pk-tabs">`)
	for i, sheet := range sheets {
		cls := "pk-tab"
		if i == 0 {
			cls += " is-active"
		}
		b.WriteString(`<button type="button" class="` + cls +
			`" data-pk-sheet="` + html.EscapeString(sheet) + `">` +
			html.EscapeString(sheet) + `</button>`)
	}
	b.WriteString(`</div>`)

	for i, sheet := range sheets {
		style := "display:none"
		if i == 0 {
			style = "display:block"
		}
		b.WriteString(`<div class="pk-sheet" data-pk-sheet="` +
			html.EscapeString(sheet) + `" style="` + style + `">`)
		if err := renderSheet(&b, f, sheet, opts); err != nil {
			return "", err
		}
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`)
	return template.HTML(b.String()), nil
}

func renderSheet(b *strings.Builder, f *excelize.File, sheet string, opts ExcelOptions) error {
	// Buffer up to MaxRows so column widths can be computed before emitting
	// the table. Excelize's sheet dimension is often "A1" for files written
	// through SetCellValue, so it is only a lower bound.
	var allRows [][]string
	rows, err := f.Rows(sheet)
	if err != nil {
		return err
	}
	for rows.Next() {
		row, err := rows.Columns()
		if err != nil {
			rows.Close()
			return err
		}
		allRows = append(allRows, row)
		if len(allRows) >= opts.MaxRows {
			break
		}
	}
	rows.Close()

	spans, skip := sheetMerges(f, sheet)

	maxCols := 0
	for _, row := range allRows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	for cell, span := range spans {
		c, _, err := excelize.CellNameToCoordinates(cell)
		if err == nil && c+span.colspan-1 > maxCols {
			maxCols = c + span.colspan - 1
		}
	}
	if dim, err := f.GetSheetDimension(sheet); err == nil {
		if c := lastColumnOfDimension(dim); c > maxCols {
			maxCols = c
		}
	}
	if maxCols > opts.MaxCols {
		maxCols = opts.MaxCols
	}
	if maxCols < 1 {
		maxCols = 1
	}

	b.WriteString(`<table class="pk-table"><colgroup>`)
	for c := 1; c <= maxCols; c++ {
		name, err := excelize.ColumnNumberToName(c)
		if err != nil {
			continue
		}
		width := 80.0
		if w, err := f.GetColWidth(sheet, name); err == nil && w > 0 {
			width = w*7 + 5
		}
		fmt.Fprintf(b, `<col style="width:%.0fpx">`, width)
	}
	b.WriteString(`</colgroup><tbody>`)

	styleCache := make(map[int]string)

	for i, row := range allRows {
		rowIdx := i + 1

		b.WriteString(`<tr`)
		if h, err := f.GetRowHeight(sheet, rowIdx); err == nil && h > 0 {
			fmt.Fprintf(b, ` style="height:%.0fpx"`, h*96/72)
		}
		b.WriteString(`>`)

		for colIdx := 1; colIdx <= len(row) && colIdx <= maxCols; colIdx++ {
			cell, err := excelize.CoordinatesToCellName(colIdx, rowIdx)
			if err != nil {
				continue
			}
			if skip[cell] {
				continue
			}

			span := spans[cell]
			b.WriteString(`<td class="pk-cell"`)
			if span.colspan > 1 {
				fmt.Fprintf(b, ` colspan="%d"`, span.colspan)
			}
			if span.rowspan > 1 {
				fmt.Fprintf(b, ` rowspan="%d"`, span.rowspan)
			}
			if css := cellStyleCSS(f, sheet, cell, styleCache); css != "" {
				b.WriteString(` style="` + html.EscapeString(css) + `"`)
			}
			b.WriteString(`>`)
			b.WriteString(html.EscapeString(row[colIdx-1]))
			b.WriteString(`</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return nil
}

func sheetMerges(f *excelize.File, sheet string) (map[string]cellSpan, map[string]bool) {
	spans := make(map[string]cellSpan)
	skip := make(map[string]bool)

	merges, err := f.GetMergeCells(sheet)
	if err != nil {
		return spans, skip
	}

	for _, m := range merges {
		startCol, startRow, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
		endCol, endRow, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
		if err1 != nil || err2 != nil {
			continue
		}

		start, err := excelize.CoordinatesToCellName(startCol, startRow)
		if err != nil {
			continue
		}
		spans[start] = cellSpan{
			rowspan: endRow - startRow + 1,
			colspan: endCol - startCol + 1,
		}

		for r := startRow; r <= endRow; r++ {
			for c := startCol; c <= endCol; c++ {
				if r == startRow && c == startCol {
					continue
				}
				name, err := excelize.CoordinatesToCellName(c, r)
				if err == nil {
					skip[name] = true
				}
			}
		}
	}
	return spans, skip
}

func cellStyleCSS(f *excelize.File, sheet, cell string, cache map[int]string) string {
	id, err := f.GetCellStyle(sheet, cell)
	if err != nil || id == 0 {
		return ""
	}
	if css, ok := cache[id]; ok {
		return css
	}

	st, err := f.GetStyle(id)
	if err != nil || st == nil {
		cache[id] = ""
		return ""
	}

	var css []string
	if st.Font != nil {
		if st.Font.Bold {
			css = append(css, "font-weight:600")
		}
		if st.Font.Italic {
			css = append(css, "font-style:italic")
		}
		if st.Font.Underline != "" {
			css = append(css, "text-decoration:underline")
		}
		if st.Font.Size > 0 {
			css = append(css, fmt.Sprintf("font-size:%.1fpt", st.Font.Size))
		}
		if c := hexColor(st.Font.Color); c != "" {
			css = append(css, "color:"+c)
		}
	}
	if len(st.Fill.Color) > 0 {
		if c := hexColor(st.Fill.Color[0]); c != "" {
			css = append(css, "background-color:"+c)
		}
	}
	if st.Alignment != nil {
		if h := mapHorizontal(st.Alignment.Horizontal); h != "" {
			css = append(css, "text-align:"+h)
		}
		if v := mapVertical(st.Alignment.Vertical); v != "" {
			css = append(css, "vertical-align:"+v)
		}
		if st.Alignment.WrapText {
			css = append(css, "white-space:pre-wrap")
		}
	}

	out := strings.Join(css, ";")
	cache[id] = out
	return out
}

func hexColor(v string) string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "#")
	if len(v) == 8 {
		v = v[2:] // drop the ARGB alpha channel
	}
	if len(v) != 6 {
		return ""
	}
	return "#" + v
}

func mapHorizontal(v string) string {
	switch strings.ToLower(v) {
	case "left", "center", "right", "justify":
		return strings.ToLower(v)
	case "centercontinuous":
		return "center"
	case "distributed":
		return "justify"
	case "fill":
		return "left"
	default:
		return ""
	}
}

func mapVertical(v string) string {
	switch strings.ToLower(v) {
	case "top", "center", "bottom", "justify":
		return strings.ToLower(v)
	case "distributed":
		return "justify"
	default:
		return ""
	}
}

func lastColumnOfDimension(dim string) int {
	if dim == "" {
		return 0
	}
	dim = strings.ReplaceAll(dim, "$", "")
	parts := strings.Split(dim, ":")
	last := parts[len(parts)-1]
	col, _, err := excelize.CellNameToCoordinates(last)
	if err != nil {
		return 0
	}
	return col
}
