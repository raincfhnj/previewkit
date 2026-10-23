// Package xlsx renders XLSX/XLSM workbooks as HTML using only the Go
// standard library. It is a preview renderer, not a full spreadsheet engine.
package xlsx

import (
	"archive/zip"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"math"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/raincfhnj/previewkit/internal/ooxml"
)

// Options controls rendering limits.
type Options struct {
	MaxRows     int
	MaxCols     int
	MaxXMLBytes int64

	// MaxImageBytes limits the size of a single embedded image in bytes.
	// Values <= 0 use 4 MiB. Unsupported formats are skipped.
	MaxImageBytes int64

	// MaxTotalImageBytes limits the combined size of all embedded images
	// inlined into the HTML fragment. Values <= 0 use 20 MiB.
	MaxTotalImageBytes int64
}

type sheetInfo struct {
	Name  string
	RelID string
}

type cell struct {
	Col      int
	Row      int
	Text     string
	Style    int
	Number   float64
	IsNumber bool
	Formula  string
}

type row struct {
	Num    int
	Height float64
	Cells  []cell
}

type worksheet struct {
	Rows             []row
	Merges           map[string]span
	Skip             map[string]bool
	Cols             map[int]float64
	Images           []sheetImage
	DefaultRowHeight float64
	MaxCol           int
	MaxRow           int
}

// sheetImage is an embedded picture positioned on a sheet.
type sheetImage struct {
	MimeType string
	Data     []byte
	X        float64
	Y        float64
	W        float64
	H        float64
}

// relationship is an OOXML package relationship.
type relationship struct {
	ID       string
	Type     string
	Target   string
	External bool
}

type span struct {
	rowspan int
	colspan int
}

type fontStyle struct {
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
	Size      float64
	Color     string
}

type fillStyle struct {
	Color string
}

type borderSide struct {
	Style string
	Color string
}

type borderStyle struct {
	Left   borderSide
	Right  borderSide
	Top    borderSide
	Bottom borderSide
}

type xfStyle struct {
	NumFmtID  int
	FontID    int
	FillID    int
	BorderID  int
	ApplyFont bool
	ApplyFill bool
	HAlign    string
	VAlign    string
	Wrap      bool
}

type styles struct {
	Fonts   []fontStyle
	Fills   []fillStyle
	Borders []borderStyle
	XFs     []xfStyle
	NumFmts map[int]string
}

// Render converts a workbook to an HTML fragment.
func Render(path string, opts Options) (template.HTML, error) {
	if opts.MaxRows <= 0 {
		opts.MaxRows = 2000
	}
	if opts.MaxCols <= 0 {
		opts.MaxCols = 100
	}
	if opts.MaxXMLBytes <= 0 {
		opts.MaxXMLBytes = 32 << 20
	}
	if opts.MaxImageBytes <= 0 {
		opts.MaxImageBytes = 4 << 20
	}
	if opts.MaxTotalImageBytes <= 0 {
		opts.MaxTotalImageBytes = 20 << 20
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	wbXML, err := ooxml.ReadZipFileLimited(zr, "xl/workbook.xml", opts.MaxXMLBytes)
	if err != nil {
		return "", fmt.Errorf("xlsx: read workbook: %w", err)
	}
	wb, err := ooxml.ParseXML(wbXML)
	if err != nil {
		return "", fmt.Errorf("xlsx: parse workbook: %w", err)
	}
	sheets := parseWorkbook(wb)
	if len(sheets) == 0 {
		return "", fmt.Errorf("xlsx: workbook has no sheets")
	}

	rels := map[string]string{}
	if data, err := ooxml.ReadZipFileLimited(zr, "xl/_rels/workbook.xml.rels", opts.MaxXMLBytes); err == nil {
		if root, err := ooxml.ParseXML(data); err == nil {
			rels = parseRelationships(root)
		}
	}

	sstPath := "xl/sharedStrings.xml"
	stylesPath := "xl/styles.xml"
	for _, target := range rels {
		lower := strings.ToLower(target)
		switch {
		case strings.Contains(lower, "sharedstrings"):
			sstPath = ooxml.ResolveTarget("xl", target)
		case strings.Contains(lower, "styles"):
			stylesPath = ooxml.ResolveTarget("xl", target)
		}
	}

	sst := []string{}
	if data, err := ooxml.ReadZipFileLimited(zr, sstPath, opts.MaxXMLBytes); err == nil {
		if root, err := ooxml.ParseXML(data); err == nil {
			sst = parseSharedStrings(root)
		}
	}

	theme := ooxml.ReadThemeColors(zr, "xl/theme/", opts.MaxXMLBytes)

	st := styles{NumFmts: map[int]string{}}
	if data, err := ooxml.ReadZipFileLimited(zr, stylesPath, opts.MaxXMLBytes); err == nil {
		if root, err := ooxml.ParseXML(data); err == nil {
			st = parseStyles(root, theme)
		}
	}

	var b strings.Builder
	b.WriteString(`<div class="pk-workbook"><div class="pk-tabs">`)
	for i, sheet := range sheets {
		cls := "pk-tab"
		if i == 0 {
			cls += " is-active"
		}
		b.WriteString(`<button type="button" class="` + cls +
			`" data-pk-sheet="` + html.EscapeString(sheet.Name) + `">` +
			html.EscapeString(sheet.Name) + `</button>`)
	}
	b.WriteString(`</div>`)

	totalImageBytes := int64(0)
	for i, sheet := range sheets {
		target := rels[sheet.RelID]
		if target == "" {
			target = fmt.Sprintf("worksheets/sheet%d.xml", i+1)
		}
		sheetPath := ooxml.ResolveTarget("xl", target)

		data, err := ooxml.ReadZipFileLimited(zr, sheetPath, opts.MaxXMLBytes)
		if err != nil {
			// Some producers use a different relationship target. Try the
			// conventional path before failing.
			fallback := fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
			data, err = ooxml.ReadZipFileLimited(zr, fallback, opts.MaxXMLBytes)
			if err != nil {
				return "", fmt.Errorf("xlsx: read sheet %q: %w", sheet.Name, err)
			}
			sheetPath = fallback
		}
		root, err := ooxml.ParseXML(data)
		if err != nil {
			return "", fmt.Errorf("xlsx: parse sheet %q: %w", sheet.Name, err)
		}
		ws := parseSheet(root, sst)
		ws.Images = parseSheetImages(zr, sheetPath, &ws, opts, &totalImageBytes)

		style := "display:none"
		if i == 0 {
			style = "display:block"
		}
		b.WriteString(`<div class="pk-sheet" data-pk-sheet="` +
			html.EscapeString(sheet.Name) + `" style="` + style + `">`)
		b.WriteString(`<div class="pk-sheet-canvas" style="position:relative">`)
		renderWorksheet(&b, ws, st, opts)
		renderSheetImages(&b, ws.Images)
		b.WriteString(`</div></div>`)
	}

	b.WriteString(`</div>`)
	return template.HTML(b.String()), nil
}

func parseWorkbook(root *ooxml.Node) []sheetInfo {
	var out []sheetInfo
	sheets := root.Child("sheets")
	if sheets == nil {
		return out
	}
	for _, s := range sheets.Children("sheet") {
		out = append(out, sheetInfo{
			Name:  s.Attr("name"),
			RelID: s.Attr("id"),
		})
	}
	return out
}

func parseRelationships(root *ooxml.Node) map[string]string {
	out := make(map[string]string)
	for _, r := range root.Children("Relationship") {
		out[r.Attr("Id")] = r.Attr("Target")
	}
	return out
}

func parseRelationshipList(root *ooxml.Node) []relationship {
	var out []relationship
	for _, r := range root.Children("Relationship") {
		out = append(out, relationship{
			ID:       r.Attr("Id"),
			Type:     r.Attr("Type"),
			Target:   r.Attr("Target"),
			External: strings.EqualFold(r.Attr("TargetMode"), "External"),
		})
	}
	return out
}

// partRelsPath returns the relationship part path for an OOXML part, for
// example "xl/worksheets/sheet1.xml" ->
// "xl/worksheets/_rels/sheet1.xml.rels".
func partRelsPath(part string) string {
	return path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
}

const (
	emuPerPx      = 9525.0
	maxDrawingCol = 16384
	maxDrawingRow = 16384
)

// parseSheetImages reads the drawing relationships of a worksheet and inlines
// the referenced pictures. Unsupported or oversized images are skipped.
func parseSheetImages(zr *zip.ReadCloser, sheetPath string, ws *worksheet, opts Options, used *int64) []sheetImage {
	data, err := ooxml.ReadZipFileLimited(zr, partRelsPath(sheetPath), opts.MaxXMLBytes)
	if err != nil {
		return nil
	}
	root, err := ooxml.ParseXML(data)
	if err != nil {
		return nil
	}

	var out []sheetImage
	for _, r := range parseRelationshipList(root) {
		if r.External || r.Target == "" {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(r.Type), "/drawing") {
			continue
		}
		drawingPath := ooxml.ResolveTarget(path.Dir(sheetPath), r.Target)
		if drawingPath == "" {
			continue
		}
		out = append(out, parseDrawingImages(zr, drawingPath, ws, opts, used)...)
	}
	return out
}

func parseDrawingImages(zr *zip.ReadCloser, drawingPath string, ws *worksheet, opts Options, used *int64) []sheetImage {
	data, err := ooxml.ReadZipFileLimited(zr, drawingPath, opts.MaxXMLBytes)
	if err != nil {
		return nil
	}
	root, err := ooxml.ParseXML(data)
	if err != nil {
		return nil
	}

	rels := map[string]relationship{}
	if relData, err := ooxml.ReadZipFileLimited(zr, partRelsPath(drawingPath), opts.MaxXMLBytes); err == nil {
		if relRoot, err := ooxml.ParseXML(relData); err == nil {
			for _, r := range parseRelationshipList(relRoot) {
				rels[r.ID] = r
			}
		}
	}

	var out []sheetImage
	for _, anchor := range root.Nodes {
		x, y, w, h, ok := anchorBox(ws, anchor)
		if !ok {
			continue
		}
		pic := findDescendant(anchor, "pic")
		if pic == nil {
			continue
		}
		blip := findDescendant(pic, "blip")
		if blip == nil {
			continue
		}
		r, ok := rels[blip.Attr("embed")]
		if !ok || r.External || r.Target == "" {
			continue
		}

		mediaPath := ooxml.ResolveTarget(path.Dir(drawingPath), r.Target)
		mimeType := imageMimeType(mediaPath)
		if mimeType == "" {
			continue
		}

		limit := opts.MaxImageBytes
		if remaining := opts.MaxTotalImageBytes - *used; remaining < limit {
			limit = remaining
		}
		if limit <= 0 {
			continue
		}
		imgData, err := ooxml.ReadZipFileLimited(zr, mediaPath, limit)
		if err != nil || len(imgData) == 0 {
			continue
		}
		*used += int64(len(imgData))
		out = append(out, sheetImage{
			MimeType: mimeType,
			Data:     imgData,
			X:        x,
			Y:        y,
			W:        w,
			H:        h,
		})
	}
	return out
}

func imageMimeType(part string) string {
	switch strings.ToLower(path.Ext(part)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return ""
	}
}

// anchorBox converts a drawing anchor into CSS pixel coordinates and size.
func anchorBox(ws *worksheet, anchor *ooxml.Node) (x, y, w, h float64, ok bool) {
	switch anchor.Name.Local {
	case "twoCellAnchor":
		from, to := anchor.Child("from"), anchor.Child("to")
		if from == nil || to == nil {
			return 0, 0, 0, 0, false
		}
		x, y = anchorPoint(ws, from)
		x2, y2 := anchorPoint(ws, to)
		w, h = x2-x, y2-y
	case "oneCellAnchor":
		from, ext := anchor.Child("from"), anchor.Child("ext")
		if from == nil || ext == nil {
			return 0, 0, 0, 0, false
		}
		x, y = anchorPoint(ws, from)
		w = ooxml.ParseFloat(ext.Attr("cx")) / emuPerPx
		h = ooxml.ParseFloat(ext.Attr("cy")) / emuPerPx
	case "absoluteAnchor":
		ext := anchor.Child("ext")
		if ext == nil {
			return 0, 0, 0, 0, false
		}
		if pos := anchor.Child("pos"); pos != nil {
			x = ooxml.ParseFloat(pos.Attr("x")) / emuPerPx
			y = ooxml.ParseFloat(pos.Attr("y")) / emuPerPx
		}
		w = ooxml.ParseFloat(ext.Attr("cx")) / emuPerPx
		h = ooxml.ParseFloat(ext.Attr("cy")) / emuPerPx
	default:
		return 0, 0, 0, 0, false
	}
	if !validPixels(x) || !validPixels(y) || !validPixels(w) || !validPixels(h) || w <= 0 || h <= 0 {
		return 0, 0, 0, 0, false
	}
	return x, y, w, h, true
}

func validPixels(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// anchorPoint computes the pixel offset of a <xdr:from>/<xdr:to> marker.
// Column and row indices are zero-based and offsets are EMU.
func anchorPoint(ws *worksheet, n *ooxml.Node) (x, y float64) {
	col := ooxml.ParseInt(n.Child("col").TextContent())
	row := ooxml.ParseInt(n.Child("row").TextContent())
	colOff := ooxml.ParseFloat(n.Child("colOff").TextContent())
	rowOff := ooxml.ParseFloat(n.Child("rowOff").TextContent())
	return colOffsetPx(ws, col) + colOff/emuPerPx,
		rowOffsetPx(ws, row) + rowOff/emuPerPx
}

func colOffsetPx(ws *worksheet, col int) float64 {
	if col < 0 {
		col = 0
	}
	if col > maxDrawingCol {
		col = maxDrawingCol
	}
	var x float64
	for i := 0; i < col; i++ {
		x += colWidthPx(ws, i)
	}
	return x
}

func colWidthPx(ws *worksheet, col int) float64 {
	width := 8.43
	if w, ok := ws.Cols[col+1]; ok && w > 0 {
		width = w
	}
	return math.Round(width*7 + 5)
}

func rowOffsetPx(ws *worksheet, row int) float64 {
	if row < 0 {
		row = 0
	}
	if row > maxDrawingRow {
		row = maxDrawingRow
	}

	heights := make(map[int]float64, len(ws.Rows))
	for _, r := range ws.Rows {
		height := r.Height
		if height <= 0 {
			height = defaultRowHeight(ws)
		}
		heights[r.Num] = math.Round(height * 96 / 72)
	}
	fallback := math.Round(defaultRowHeight(ws) * 96 / 72)

	var y float64
	for i := 1; i <= row; i++ {
		if height, ok := heights[i]; ok {
			y += height
		} else {
			y += fallback
		}
	}
	return y
}

func defaultRowHeight(ws *worksheet) float64 {
	if ws.DefaultRowHeight > 0 {
		return ws.DefaultRowHeight
	}
	return 15
}

func findDescendant(n *ooxml.Node, local string) *ooxml.Node {
	if n == nil {
		return nil
	}
	if n.Name.Local == local {
		return n
	}
	for _, c := range n.Nodes {
		if got := findDescendant(c, local); got != nil {
			return got
		}
	}
	return nil
}

func parseSharedStrings(root *ooxml.Node) []string {
	var out []string
	for _, si := range root.Children("si") {
		out = append(out, textFromNode(si))
	}
	return out
}

func textFromNode(n *ooxml.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	if t := n.Child("t"); t != nil {
		b.WriteString(t.TextContent())
	}
	for _, r := range n.Children("r") {
		if t := r.Child("t"); t != nil {
			b.WriteString(t.TextContent())
		}
	}
	return b.String()
}

func parseStyles(root *ooxml.Node, theme map[string]string) styles {
	st := styles{NumFmts: map[int]string{}}

	if nf := root.Child("numFmts"); nf != nil {
		for _, n := range nf.Children("numFmt") {
			id := ooxml.ParseInt(n.Attr("numFmtId"))
			st.NumFmts[id] = n.Attr("formatCode")
		}
	}

	if fonts := root.Child("fonts"); fonts != nil {
		for _, f := range fonts.Children("font") {
			var fs fontStyle
			fs.Bold = onOff(f.Child("b"))
			fs.Italic = onOff(f.Child("i"))
			if u := f.Child("u"); u != nil {
				fs.Underline = u.Attr("val") != "none"
			}
			fs.Strike = onOff(f.Child("strike"))
			if sz := f.Child("sz"); sz != nil {
				fs.Size = ooxml.ParseFloat(sz.Attr("val"))
			}
			if c := f.Child("color"); c != nil {
				fs.Color = colorFromNode(c, theme)
			}
			st.Fonts = append(st.Fonts, fs)
		}
	}

	if fills := root.Child("fills"); fills != nil {
		for _, f := range fills.Children("fill") {
			var fs fillStyle
			if pf := f.Child("patternFill"); pf != nil {
				if pf.Attr("patternType") == "solid" {
					if c := pf.Child("fgColor"); c != nil {
						fs.Color = colorFromNode(c, theme)
					}
				}
			}
			st.Fills = append(st.Fills, fs)
		}
	}

	if borders := root.Child("borders"); borders != nil {
		for _, b := range borders.Children("border") {
			st.Borders = append(st.Borders, borderStyle{
				Left:   parseBorderSide(b.Child("left"), theme),
				Right:  parseBorderSide(b.Child("right"), theme),
				Top:    parseBorderSide(b.Child("top"), theme),
				Bottom: parseBorderSide(b.Child("bottom"), theme),
			})
		}
	}

	if xfs := root.Child("cellXfs"); xfs != nil {
		for _, x := range xfs.Children("xf") {
			xf := xfStyle{
				NumFmtID:  ooxml.ParseInt(x.Attr("numFmtId")),
				FontID:    ooxml.ParseInt(x.Attr("fontId")),
				FillID:    ooxml.ParseInt(x.Attr("fillId")),
				BorderID:  ooxml.ParseInt(x.Attr("borderId")),
				ApplyFont: ooxml.ParseBool(x.Attr("applyFont")),
				ApplyFill: ooxml.ParseBool(x.Attr("applyFill")),
			}
			if a := x.Child("alignment"); a != nil {
				xf.HAlign = a.Attr("horizontal")
				xf.VAlign = a.Attr("vertical")
				xf.Wrap = ooxml.ParseBool(a.Attr("wrapText"))
			}
			st.XFs = append(st.XFs, xf)
		}
	}

	return st
}

func parseBorderSide(n *ooxml.Node, theme map[string]string) borderSide {
	if n == nil {
		return borderSide{}
	}
	style := n.Attr("style")
	if style == "" || style == "none" {
		return borderSide{}
	}
	color := ""
	if c := n.Child("color"); c != nil {
		color = colorFromNode(c, theme)
	}
	if color == "" {
		color = "#000000"
	}
	return borderSide{Style: style, Color: color}
}

func colorFromNode(n *ooxml.Node, theme map[string]string) string {
	if n == nil {
		return ""
	}
	if color := ooxml.HexColor(n.Attr("rgb")); color != "" {
		return color
	}
	if idx := n.Attr("theme"); idx != "" {
		return theme[idx]
	}
	if c := n.Child("srgbClr"); c != nil {
		return ooxml.HexColor(c.Attr("val"))
	}
	if c := n.Child("schemeClr"); c != nil {
		return theme[c.Attr("val")]
	}
	return ""
}

func onOff(n *ooxml.Node) bool {
	if n == nil {
		return false
	}
	v := n.Attr("val")
	return v == "" || ooxml.ParseBool(v)
}

func parseSheet(root *ooxml.Node, sst []string) worksheet {
	ws := worksheet{
		Merges: make(map[string]span),
		Skip:   make(map[string]bool),
		Cols:   make(map[int]float64),
	}

	if cols := root.Child("cols"); cols != nil {
		for _, c := range cols.Children("col") {
			min := ooxml.ParseInt(c.Attr("min"))
			max := ooxml.ParseInt(c.Attr("max"))
			width := ooxml.ParseFloat(c.Attr("width"))
			if min <= 0 || max < min || width <= 0 {
				continue
			}
			for i := min; i <= max && i <= 2000; i++ {
				ws.Cols[i] = width
			}
		}
	}
	if sf := root.Child("sheetFormatPr"); sf != nil {
		ws.DefaultRowHeight = ooxml.ParseFloat(sf.Attr("defaultRowHeight"))
	}

	if merges := root.Child("mergeCells"); merges != nil {
		for _, m := range merges.Children("mergeCell") {
			ref := m.Attr("ref")
			parts := strings.SplitN(ref, ":", 2)
			if len(parts) != 2 {
				continue
			}
			startCol, startRow, ok1 := ooxml.ParseCellRef(parts[0])
			endCol, endRow, ok2 := ooxml.ParseCellRef(parts[1])
			if !ok1 || !ok2 || endCol < startCol || endRow < startRow {
				continue
			}
			start := ooxml.CellName(startCol, startRow)
			ws.Merges[start] = span{rowspan: endRow - startRow + 1, colspan: endCol - startCol + 1}
			for r := startRow; r <= endRow; r++ {
				for c := startCol; c <= endCol; c++ {
					if r == startRow && c == startCol {
						continue
					}
					ws.Skip[ooxml.CellName(c, r)] = true
				}
			}
			if endCol > ws.MaxCol {
				ws.MaxCol = endCol
			}
			if endRow > ws.MaxRow {
				ws.MaxRow = endRow
			}
		}
	}

	sheetData := root.Child("sheetData")
	if sheetData == nil {
		return ws
	}

	prevRow := 0
	for _, rn := range sheetData.Children("row") {
		rnum := ooxml.ParseInt(rn.Attr("r"))
		if rnum <= 0 {
			rnum = prevRow + 1
		}
		prevRow = rnum
		ws.MaxRow = maxInt(ws.MaxRow, rnum)

		r := row{Num: rnum, Height: ooxml.ParseFloat(rn.Attr("ht"))}
		prevCol := 0
		for _, cn := range rn.Children("c") {
			col, rowNum, ok := ooxml.ParseCellRef(cn.Attr("r"))
			if !ok {
				col = prevCol + 1
				rowNum = rnum
			}
			prevCol = col
			if rowNum <= 0 {
				rowNum = rnum
			}
			ws.MaxCol = maxInt(ws.MaxCol, col)
			ws.MaxRow = maxInt(ws.MaxRow, rowNum)
			r.Cells = append(r.Cells, parseCell(cn, col, rowNum, sst))
		}
		ws.Rows = append(ws.Rows, r)
	}

	return ws
}

func parseCell(n *ooxml.Node, col, row int, sst []string) cell {
	c := cell{
		Col:   col,
		Row:   row,
		Style: ooxml.ParseInt(n.Attr("s")),
	}

	typ := n.Attr("t")
	v := ""
	if vn := n.Child("v"); vn != nil {
		v = vn.TextContent()
	}
	if f := n.Child("f"); f != nil {
		c.Formula = f.TextContent()
	}

	switch typ {
	case "s":
		idx := ooxml.ParseInt(v)
		if idx >= 0 && idx < len(sst) {
			c.Text = sst[idx]
		}
	case "inlineStr":
		c.Text = textFromNode(n.Child("is"))
	case "b":
		if v == "1" {
			c.Text = "TRUE"
		} else {
			c.Text = "FALSE"
		}
	case "e", "str", "d":
		c.Text = v
	case "", "n":
		if v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err == nil {
				c.Number = f
				c.IsNumber = true
			} else {
				c.Text = v
			}
		} else if c.Formula != "" {
			c.Text = "=" + c.Formula
		}
	default:
		c.Text = v
	}

	return c
}

func renderWorksheet(b *strings.Builder, ws worksheet, st styles, opts Options) {
	maxCols := ws.MaxCol
	if maxCols > opts.MaxCols {
		maxCols = opts.MaxCols
	}
	if maxCols < 1 {
		maxCols = 1
	}

	b.WriteString(`<table class="pk-table"><colgroup>`)
	for c := 1; c <= maxCols; c++ {
		width := 8.43
		if w, ok := ws.Cols[c]; ok {
			width = w
		}
		fmt.Fprintf(b, `<col style="width:%.0fpx">`, width*7+5)
	}
	b.WriteString(`</colgroup><tbody>`)

	defaultHeight := ws.DefaultRowHeight
	if defaultHeight <= 0 {
		defaultHeight = 15
	}

	rendered := 0
	for _, r := range ws.Rows {
		if rendered >= opts.MaxRows {
			break
		}
		rendered++

		height := r.Height
		if height <= 0 {
			height = defaultHeight
		}
		fmt.Fprintf(b, `<tr style="height:%.0fpx">`, height*96/72)

		byCol := make(map[int]cell, len(r.Cells))
		for _, c := range r.Cells {
			byCol[c.Col] = c
		}

		for col := 1; col <= maxCols; col++ {
			name := ooxml.CellName(col, r.Num)
			if ws.Skip[name] {
				continue
			}
			c, ok := byCol[col]
			if !ok {
				b.WriteString(`<td class="pk-cell"></td>`)
				continue
			}

			sp := ws.Merges[name]
			colspan := sp.colspan
			rowspan := sp.rowspan
			if colspan < 1 {
				colspan = 1
			}
			if rowspan < 1 {
				rowspan = 1
			}
			if col+colspan-1 > maxCols {
				colspan = maxCols - col + 1
			}

			b.WriteString(`<td class="pk-cell"`)
			if colspan > 1 {
				fmt.Fprintf(b, ` colspan="%d"`, colspan)
			}
			if rowspan > 1 {
				fmt.Fprintf(b, ` rowspan="%d"`, rowspan)
			}
			if css := styleCSS(c.Style, st); css != "" {
				b.WriteString(` style="` + html.EscapeString(css) + `"`)
			}
			b.WriteString(`>`)
			b.WriteString(html.EscapeString(cellText(c, st)))
			b.WriteString(`</td>`)
		}
		b.WriteString(`</tr>`)
	}

	b.WriteString(`</tbody></table>`)
}

func renderSheetImages(b *strings.Builder, images []sheetImage) {
	for _, img := range images {
		fmt.Fprintf(b, `<img class="pk-sheet-img" alt="" src="data:%s;base64,%s" style="position:absolute;left:%.0fpx;top:%.0fpx;width:%.0fpx;height:%.0fpx">`,
			img.MimeType,
			base64.StdEncoding.EncodeToString(img.Data),
			img.X, img.Y, img.W, img.H)
	}
}

func cellText(c cell, st styles) string {
	if !c.IsNumber {
		return c.Text
	}

	xf := st.xf(c.Style)
	code := st.NumFmts[xf.NumFmtID]
	if isDateFormat(xf.NumFmtID, code) {
		return formatExcelDate(c.Number)
	}
	if isPercentFormat(xf.NumFmtID, code) {
		return trimFloat(c.Number*100) + "%"
	}
	if strings.Contains(code, "0.00") {
		return strconv.FormatFloat(c.Number, 'f', 2, 64)
	}
	return trimFloat(c.Number)
}

func trimFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return ""
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if s == "-0" {
		return "0"
	}
	return s
}

func formatExcelDate(serial float64) string {
	days := int(math.Floor(serial))
	frac := serial - float64(days)
	if days > 59 {
		days--
	}
	t := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC).AddDate(0, 0, days)
	if frac <= 0 {
		return t.Format("2006-01-02")
	}
	seconds := int(math.Round(frac * 86400))
	t = t.Add(time.Duration(seconds) * time.Second)
	return t.Format("2006-01-02 15:04")
}

func isDateFormat(id int, code string) bool {
	switch id {
	case 14, 15, 16, 17, 18, 19, 20, 21, 22, 45, 46, 47:
		return true
	}
	code = strings.ToLower(code)
	if strings.Contains(code, "\"") {
		var b strings.Builder
		inQuote := false
		for i := 0; i < len(code); i++ {
			if code[i] == '"' {
				inQuote = !inQuote
				continue
			}
			if !inQuote {
				b.WriteByte(code[i])
			}
		}
		code = b.String()
	}
	return strings.ContainsAny(code, "ymdhs")
}

func isPercentFormat(id int, code string) bool {
	return id == 9 || id == 10 || strings.Contains(code, "%")
}

func styleCSS(id int, st styles) string {
	xf := st.xf(id)
	var css []string

	if xf.FontID >= 0 && xf.FontID < len(st.Fonts) {
		f := st.Fonts[xf.FontID]
		if f.Bold {
			css = append(css, "font-weight:600")
		}
		if f.Italic {
			css = append(css, "font-style:italic")
		}
		var deco []string
		if f.Underline {
			deco = append(deco, "underline")
		}
		if f.Strike {
			deco = append(deco, "line-through")
		}
		if len(deco) > 0 {
			css = append(css, "text-decoration:"+strings.Join(deco, " "))
		}
		if f.Size > 0 {
			css = append(css, fmt.Sprintf("font-size:%.1fpt", f.Size))
		}
		if f.Color != "" {
			css = append(css, "color:"+f.Color)
		}
	}
	if xf.FillID >= 0 && xf.FillID < len(st.Fills) && st.Fills[xf.FillID].Color != "" {
		css = append(css, "background-color:"+st.Fills[xf.FillID].Color)
	}
	if h := mapHorizontal(xf.HAlign); h != "" {
		css = append(css, "text-align:"+h)
	}
	if v := mapVertical(xf.VAlign); v != "" {
		css = append(css, "vertical-align:"+v)
	}
	if xf.Wrap {
		css = append(css, "white-space:pre-wrap")
	}
	if xf.BorderID >= 0 && xf.BorderID < len(st.Borders) {
		b := st.Borders[xf.BorderID]
		css = append(css, borderCSS("border-left", b.Left)...)
		css = append(css, borderCSS("border-right", b.Right)...)
		css = append(css, borderCSS("border-top", b.Top)...)
		css = append(css, borderCSS("border-bottom", b.Bottom)...)
	}
	return strings.Join(css, ";")
}

func borderCSS(property string, side borderSide) []string {
	if side.Style == "" {
		return nil
	}
	width := "1px"
	style := "solid"
	switch strings.ToLower(side.Style) {
	case "medium":
		width = "2px"
	case "thick":
		width = "3px"
	case "double":
		width = "3px"
		style = "double"
	case "dashed", "dashdot", "dashdotdot":
		style = "dashed"
	case "dotted":
		style = "dotted"
	case "hair":
		width = "1px"
	}
	return []string{property + ":" + width + " " + style + " " + side.Color}
}

func (st styles) xf(id int) xfStyle {
	if id >= 0 && id < len(st.XFs) {
		return st.XFs[id]
	}
	return xfStyle{}
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
