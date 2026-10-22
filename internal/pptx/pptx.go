// Package pptx renders PPTX presentations as HTML using only the Go standard
// library.
//
// This is a content preview: text boxes, basic shapes, tables, pictures, and
// slide backgrounds are supported. Charts, SmartArt, animations, transitions,
// and exact theme/master inheritance are not.
package pptx

import (
	"archive/zip"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"math"
	"path"
	"strings"

	"github.com/raincfhnj/previewkit/internal/ooxml"
)

const (
	relNS         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	emuPerPixel   = 9525
	defaultSlideW = 12192000
	defaultSlideH = 6858000
)

// Options controls rendering limits and output width.
type Options struct {
	TargetWidth        int
	MaxXMLBytes        int64
	MaxImageBytes      int
	MaxTotalImageBytes int
}

type relInfo struct {
	Target   string
	External bool
}

type rect struct {
	x, y, w, h float64
}

type renderer struct {
	zr      *zip.ReadCloser
	opts    Options
	slideCX int
	slideCY int
	scale   float64
	slideH  int

	imageTotal int
	curRels    map[string]relInfo
}

type slideRef struct {
	path string
}

// Render converts a PPTX presentation to an HTML fragment.
func Render(filePath string, opts Options) (template.HTML, error) {
	if opts.TargetWidth <= 0 {
		opts.TargetWidth = 960
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

	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	presXML, err := ooxml.ReadZipFileLimited(zr, "ppt/presentation.xml", opts.MaxXMLBytes)
	if err != nil {
		return "", fmt.Errorf("pptx: read presentation: %w", err)
	}
	pres, err := ooxml.ParseXML(presXML)
	if err != nil {
		return "", fmt.Errorf("pptx: parse presentation: %w", err)
	}

	slideCX, slideCY := defaultSlideW, defaultSlideH
	if sz := pres.Child("sldSz"); sz != nil {
		if v := ooxml.ParseInt(sz.Attr("cx")); v > 0 {
			slideCX = v
		}
		if v := ooxml.ParseInt(sz.Attr("cy")); v > 0 {
			slideCY = v
		}
	}

	rels := readRels(zr, "ppt/presentation.xml", opts.MaxXMLBytes)
	var slides []slideRef
	if lst := pres.Child("sldIdLst"); lst != nil {
		for _, s := range lst.Children("sldId") {
			rid := relID(s)
			target := rels[rid].Target
			if target == "" {
				continue
			}
			slides = append(slides, slideRef{
				path: ooxml.ResolveTarget("ppt", target),
			})
		}
	}

	scale := float64(opts.TargetWidth) / (float64(slideCX) / emuPerPixel)
	r := &renderer{
		zr:      zr,
		opts:    opts,
		slideCX: slideCX,
		slideCY: slideCY,
		scale:   scale,
		slideH:  int(math.Round(float64(slideCY) / emuPerPixel * scale)),
	}

	var b strings.Builder
	b.WriteString(`<div class="pk-pptx">`)
	for i, s := range slides {
		if err := r.renderSlide(&b, i, s.path); err != nil {
			return "", err
		}
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String()), nil
}

func readRels(zr *zip.ReadCloser, part string, limit int64) map[string]relInfo {
	out := map[string]relInfo{}
	data, err := ooxml.ReadZipFileLimited(zr, relsPath(part), limit)
	if err != nil {
		return out
	}
	root, err := ooxml.ParseXML(data)
	if err != nil {
		return out
	}
	for _, n := range root.Children("Relationship") {
		out[n.Attr("Id")] = relInfo{
			Target:   n.Attr("Target"),
			External: strings.EqualFold(n.Attr("TargetMode"), "External"),
		}
	}
	return out
}

func relsPath(part string) string {
	return path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
}

func relID(n *ooxml.Node) string {
	if id := n.AttrNS(relNS, "id"); id != "" {
		return id
	}
	// Some producers omit the namespace declaration in test fixtures.
	for _, a := range n.Attrs {
		if a.Name.Local == "id" && strings.HasPrefix(a.Value, "rId") {
			return a.Value
		}
	}
	return ""
}

func (r *renderer) renderSlide(b *strings.Builder, index int, slidePath string) error {
	data, err := ooxml.ReadZipFileLimited(r.zr, slidePath, r.opts.MaxXMLBytes)
	if err != nil {
		return fmt.Errorf("pptx: read %s: %w", slidePath, err)
	}
	root, err := ooxml.ParseXML(data)
	if err != nil {
		return fmt.Errorf("pptx: parse %s: %w", slidePath, err)
	}

	rels := readRels(r.zr, slidePath, r.opts.MaxXMLBytes)
	layout := r.readLayoutPlaceholders(rels)

	prevRels := r.curRels
	r.curRels = rels
	defer func() { r.curRels = prevRels }()

	bg := slideBackground(root)
	fmt.Fprintf(b, `<section class="pk-pptx-slide" data-slide="%d" style="width:%dpx;height:%dpx;background:%s">`,
		index+1, r.opts.TargetWidth, r.slideH, bg)
	fmt.Fprintf(b, `<div class="pk-pptx-page">%d</div>`, index+1)

	if cSld := root.Child("cSld"); cSld != nil {
		if spTree := cSld.Child("spTree"); spTree != nil {
			r.renderSpTree(b, spTree, rels, layout)
		}
	}
	b.WriteString(`</section>`)
	return nil
}

func slideBackground(root *ooxml.Node) string {
	cSld := root.Child("cSld")
	if cSld == nil {
		return "#FFFFFF"
	}
	bg := cSld.Child("bg")
	if bg == nil {
		return "#FFFFFF"
	}
	bgPr := bg.Child("bgPr")
	if bgPr == nil {
		return "#FFFFFF"
	}
	if c := solidColor(bgPr); c != "" {
		return c
	}
	return "#FFFFFF"
}

func solidColor(n *ooxml.Node) string {
	sf := n.Child("solidFill")
	if sf == nil {
		return ""
	}
	if c := sf.Child("srgbClr"); c != nil {
		return ooxml.HexColor(c.Attr("val"))
	}
	return ""
}

func (r *renderer) renderSpTree(b *strings.Builder, spTree *ooxml.Node, rels map[string]relInfo, layout map[string]rect) {
	for _, n := range spTree.Nodes {
		switch n.Name.Local {
		case "sp":
			r.renderShape(b, n, layout)
		case "pic":
			r.renderPicture(b, n, rels)
		case "graphicFrame":
			r.renderGraphicFrame(b, n)
		case "grpSp":
			r.renderSpTree(b, n, rels, layout)
		}
	}
}

func (r *renderer) renderShape(b *strings.Builder, n *ooxml.Node, layout map[string]rect) {
	spPr := n.Child("spPr")
	rc, ok := shapeRect(r, spPr, n, layout)
	if !ok {
		return
	}

	style := r.shapeStyle(rc)
	if fill := solidColor(spPr); fill != "" {
		style += "background:" + fill + ";"
	}
	if ln := spPr.Child("ln"); ln != nil {
		if c := solidColor(ln); c != "" {
			width := 1.0
			if w := ooxml.ParseInt(ln.Attr("w")); w > 0 {
				width = float64(w) / emuPerPixel * r.scale
			}
			style += fmt.Sprintf("border:%.1fpx solid %s;", math.Max(1, width), c)
		}
	}
	if prst := spPr.Child("prstGeom"); prst != nil {
		switch prst.Attr("prst") {
		case "ellipse":
			style += "border-radius:50%;"
		case "roundRect":
			style += "border-radius:8px;"
		}
	}

	fmt.Fprintf(b, `<div class="pk-pptx-shape" style="%s">`, html.EscapeString(style))
	if txBody := n.Child("txBody"); txBody != nil {
		r.renderTextBody(b, txBody, rc, true)
	}
	b.WriteString(`</div>`)
}

func (r *renderer) renderPicture(b *strings.Builder, n *ooxml.Node, rels map[string]relInfo) {
	spPr := n.Child("spPr")
	rc, ok := xfrmRect(r, spPr.Child("xfrm"))
	if !ok {
		return
	}
	blipFill := n.Child("blipFill")
	if blipFill == nil {
		return
	}
	blip := blipFill.Child("blip")
	if blip == nil {
		return
	}
	relID := blip.Attr("embed")
	if relID == "" {
		relID = blip.AttrNS(relNS, "embed")
	}
	src := r.imageData(rels[relID])
	if src == "" {
		return
	}
	fmt.Fprintf(b, `<img class="pk-pptx-img" src="%s" alt="" style="left:%.1fpx;top:%.1fpx;width:%.1fpx;height:%.1fpx">`,
		src, rc.x, rc.y, rc.w, rc.h)
}

func (r *renderer) renderGraphicFrame(b *strings.Builder, n *ooxml.Node) {
	xfrm := n.Child("xfrm")
	rc, ok := xfrmRect(r, xfrm)
	if !ok {
		return
	}
	tbl := findFirst(n, "tbl")
	if tbl == nil {
		return
	}

	fmt.Fprintf(b, `<div class="pk-pptx-shape" style="%s">`, html.EscapeString(r.shapeStyle(rc)))
	b.WriteString(`<table class="pk-pptx-table">`)
	for _, tr := range tbl.Children("tr") {
		b.WriteString("<tr>")
		for _, tc := range tr.Children("tc") {
			colspan := ooxml.ParseInt(tc.Attr("gridSpan"))
			rowspan := ooxml.ParseInt(tc.Attr("rowSpan"))
			b.WriteString("<td")
			if colspan > 1 {
				fmt.Fprintf(b, ` colspan="%d"`, colspan)
			}
			if rowspan > 1 {
				fmt.Fprintf(b, ` rowspan="%d"`, rowspan)
			}
			if tcPr := tc.Child("tcPr"); tcPr != nil {
				if c := solidColor(tcPr); c != "" {
					b.WriteString(` style="background:` + c + `"`)
				}
			}
			b.WriteString(">")
			if txBody := tc.Child("txBody"); txBody != nil {
				r.renderParagraphs(b, txBody, false)
			}
			b.WriteString("</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString(`</table></div>`)
}

func (r *renderer) shapeStyle(rc rect) string {
	return fmt.Sprintf("left:%.1fpx;top:%.1fpx;width:%.1fpx;height:%.1fpx;", rc.x, rc.y, rc.w, rc.h)
}

func shapeRect(r *renderer, spPr, shape *ooxml.Node, layout map[string]rect) (rect, bool) {
	if spPr != nil {
		if rc, ok := xfrmRect(r, spPr.Child("xfrm")); ok {
			return rc, true
		}
	}
	if shape != nil {
		if ph := placeholder(shape); ph != nil {
			if rc, ok := layout[phKey(ph)]; ok {
				return rc, true
			}
			typ := ph.Attr("type")
			return defaultPlaceholderRect(r, typ), true
		}
	}
	return rect{}, false
}

func xfrmRect(r *renderer, xfrm *ooxml.Node) (rect, bool) {
	if xfrm == nil {
		return rect{}, false
	}
	off := xfrm.Child("off")
	ext := xfrm.Child("ext")
	if off == nil || ext == nil {
		return rect{}, false
	}
	x := ooxml.ParseInt(off.Attr("x"))
	y := ooxml.ParseInt(off.Attr("y"))
	w := ooxml.ParseInt(ext.Attr("cx"))
	h := ooxml.ParseInt(ext.Attr("cy"))
	if w <= 0 || h <= 0 {
		return rect{}, false
	}
	return rect{
		x: float64(x) / emuPerPixel * r.scale,
		y: float64(y) / emuPerPixel * r.scale,
		w: float64(w) / emuPerPixel * r.scale,
		h: float64(h) / emuPerPixel * r.scale,
	}, true
}

func placeholder(shape *ooxml.Node) *ooxml.Node {
	nv := shape.Child("nvSpPr")
	if nv == nil {
		return nil
	}
	pr := nv.Child("nvPr")
	if pr == nil {
		return nil
	}
	return pr.Child("ph")
}

func phKey(ph *ooxml.Node) string {
	return ph.Attr("type") + ":" + ph.Attr("idx")
}

func defaultPlaceholderRect(r *renderer, typ string) rect {
	slideW := float64(r.opts.TargetWidth)
	slideH := float64(r.slideH)
	switch typ {
	case "title", "ctrTitle":
		return rect{x: slideW * .06, y: slideH * .05, w: slideW * .88, h: slideH * .18}
	default:
		return rect{x: slideW * .06, y: slideH * .28, w: slideW * .88, h: slideH * .62}
	}
}

func (r *renderer) readLayoutPlaceholders(rels map[string]relInfo) map[string]rect {
	out := map[string]rect{}
	for _, rel := range rels {
		lower := strings.ToLower(rel.Target)
		if !strings.Contains(lower, "slidelayout") {
			continue
		}
		layoutPath := ooxml.ResolveTarget("ppt/slides", rel.Target)
		data, err := ooxml.ReadZipFileLimited(r.zr, layoutPath, r.opts.MaxXMLBytes)
		if err != nil {
			layoutPath = ooxml.ResolveTarget("ppt", rel.Target)
			data, err = ooxml.ReadZipFileLimited(r.zr, layoutPath, r.opts.MaxXMLBytes)
			if err != nil {
				continue
			}
		}
		root, err := ooxml.ParseXML(data)
		if err != nil {
			continue
		}
		cSld := root.Child("cSld")
		if cSld == nil {
			continue
		}
		spTree := cSld.Child("spTree")
		if spTree == nil {
			continue
		}
		for _, sp := range spTree.Children("sp") {
			ph := placeholder(sp)
			if ph == nil {
				continue
			}
			spPr := sp.Child("spPr")
			if spPr == nil {
				continue
			}
			if rc, ok := xfrmRect(r, spPr.Child("xfrm")); ok {
				out[phKey(ph)] = rc
			}
		}
	}
	return out
}

func (r *renderer) renderTextBody(b *strings.Builder, txBody *ooxml.Node, rc rect, flex bool) {
	bodyPr := txBody.Child("bodyPr")
	style := "white-space:pre-wrap;overflow:hidden;"
	if flex {
		style += "display:flex;flex-direction:column;"
		anchor := ""
		if bodyPr != nil {
			anchor = bodyPr.Attr("anchor")
		}
		switch anchor {
		case "ctr":
			style += "justify-content:center;"
		case "b":
			style += "justify-content:flex-end;"
		}
	}
	if bodyPr != nil {
		inset := func(name string, def int) float64 {
			v := ooxml.ParseInt(bodyPr.Attr(name))
			if v <= 0 {
				v = def
			}
			return float64(v) / emuPerPixel * r.scale
		}
		style += fmt.Sprintf("padding:%.1fpx %.1fpx %.1fpx %.1fpx;",
			inset("tIns", 45720), inset("rIns", 45720),
			inset("bIns", 45720), inset("lIns", 91440))
	}
	fmt.Fprintf(b, `<div class="pk-pptx-text" style="%s">`, html.EscapeString(style))
	r.renderParagraphs(b, txBody, true)
	b.WriteString(`</div>`)
}

func (r *renderer) renderParagraphs(b *strings.Builder, parent *ooxml.Node, flex bool) {
	for _, p := range parent.Children("p") {
		r.renderParagraph(b, p)
	}
}

func (r *renderer) renderParagraph(b *strings.Builder, p *ooxml.Node) {
	pPr := p.Child("pPr")
	align := ""
	style := "margin:0 0 0.35em 0;"
	if pPr != nil {
		switch pPr.Attr("algn") {
		case "ctr":
			align = "center"
		case "r":
			align = "right"
		case "just":
			align = "justify"
		case "l":
			align = "left"
		}
		if v := ooxml.ParseInt(pPr.Attr("marL")); v > 0 {
			style += fmt.Sprintf("margin-left:%.1fpx;", float64(v)/emuPerPixel*r.scale)
		}
		if v := ooxml.ParseInt(pPr.Attr("indent")); v != 0 {
			style += fmt.Sprintf("text-indent:%.1fpx;", float64(v)/emuPerPixel*r.scale)
		}
		if sp := pPr.Child("spcBef"); sp != nil {
			if pts := sp.Child("spcPts"); pts != nil {
				style += fmt.Sprintf("margin-top:%.1fpx;", float64(ooxml.ParseInt(pts.Attr("val")))/100*96/72*r.scale)
			}
		}
		if sp := pPr.Child("spcAft"); sp != nil {
			if pts := sp.Child("spcPts"); pts != nil {
				style += fmt.Sprintf("margin-bottom:%.1fpx;", float64(ooxml.ParseInt(pts.Attr("val")))/100*96/72*r.scale)
			}
		}
		if align != "" {
			style += "text-align:" + align + ";"
		}
	}

	bullet := ""
	if pPr != nil {
		if bc := pPr.Child("buChar"); bc != nil {
			bullet = bc.Attr("char")
			if bullet == "" {
				bullet = "•"
			}
		} else if ba := pPr.Child("buAutoNum"); ba != nil {
			bullet = "•"
		}
	}

	def := runStyle{fontPx: 18 * 96 / 72 * r.scale}
	if pPr != nil {
		if dr := pPr.Child("defRPr"); dr != nil {
			def = r.parseRunStyle(dr, def)
		}
	}

	b.WriteString(`<p style="` + html.EscapeString(style) + `">`)
	if bullet != "" {
		b.WriteString(`<span class="pk-pptx-bullet">` + html.EscapeString(bullet) + ` </span>`)
	}
	for _, n := range p.Nodes {
		switch n.Name.Local {
		case "r":
			b.WriteString(r.renderRun(n, def))
		case "br":
			b.WriteString("<br>")
		case "fld":
			b.WriteString(r.renderRun(n, def))
		}
	}
	b.WriteString(`</p>`)
}

type runStyle struct {
	fontPx    float64
	bold      bool
	italic    bool
	under     bool
	strike    bool
	color     string
	highlight string
	font      string
}

func (r *renderer) parseRunStyle(rPr *ooxml.Node, base runStyle) runStyle {
	out := base
	if rPr == nil {
		return out
	}
	if v := ooxml.ParseInt(rPr.Attr("sz")); v > 0 {
		out.fontPx = float64(v) / 100 * 96 / 72 * r.scale
	}
	out.bold = rPr.Attr("b") == "1" || rPr.Attr("b") == "true"
	out.italic = rPr.Attr("i") == "1" || rPr.Attr("i") == "true"
	if u := rPr.Attr("u"); u != "" && u != "none" {
		out.under = true
	}
	if s := rPr.Attr("strike"); s != "" && s != "noStrike" {
		out.strike = true
	}
	if c := solidColor(rPr); c != "" {
		out.color = c
	}
	if hl := rPr.Child("highlight"); hl != nil {
		if c := hl.Child("srgbClr"); c != nil {
			out.highlight = ooxml.HexColor(c.Attr("val"))
		}
	}
	if latin := rPr.Child("latin"); latin != nil {
		out.font = safeFont(latin.Attr("typeface"))
	}
	return out
}

func (r *renderer) renderRun(n *ooxml.Node, base runStyle) string {
	rPr := n.Child("rPr")
	st := r.parseRunStyle(rPr, base)

	text := ""
	if t := n.Child("t"); t != nil {
		text = t.TextContent()
	}
	if text == "" {
		return ""
	}

	var css []string
	css = append(css, fmt.Sprintf("font-size:%.1fpx", st.fontPx))
	if st.bold {
		css = append(css, "font-weight:700")
	}
	if st.italic {
		css = append(css, "font-style:italic")
	}
	var deco []string
	if st.under {
		deco = append(deco, "underline")
	}
	if st.strike {
		deco = append(deco, "line-through")
	}
	if len(deco) > 0 {
		css = append(css, "text-decoration:"+strings.Join(deco, " "))
	}
	if st.color != "" {
		css = append(css, "color:"+st.color)
	}
	if st.highlight != "" {
		css = append(css, "background-color:"+st.highlight)
	}
	if st.font != "" {
		css = append(css, "font-family:'"+st.font+"'")
	}

	inner := `<span style="` + html.EscapeString(strings.Join(css, ";")) + `">` +
		html.EscapeString(text) + `</span>`

	if rPr != nil {
		if hl := rPr.Child("hlinkClick"); hl != nil {
			if href := r.hyperlink(hl); href != "" {
				return `<a href="` + html.EscapeString(href) +
					`" target="_blank" rel="noopener noreferrer">` + inner + `</a>`
			}
		}
	}
	return inner
}

func (r *renderer) hyperlink(n *ooxml.Node) string {
	rid := n.AttrNS(relNS, "id")
	if rid == "" {
		rid = n.Attr("id")
	}
	rel, ok := r.curRels[rid]
	if !ok || !rel.External {
		return ""
	}
	if !safeHref(rel.Target) {
		return ""
	}
	return rel.Target
}

func (r *renderer) imageData(rel relInfo) string {
	if rel.Target == "" || rel.External {
		return ""
	}
	zipPath := ooxml.ResolveTarget("ppt/slides", rel.Target)
	data, err := ooxml.ReadZipFileLimited(r.zr, zipPath, int64(r.opts.MaxImageBytes))
	if err != nil || len(data) == 0 {
		return ""
	}
	r.imageTotal += len(data)
	if r.imageTotal > r.opts.MaxTotalImageBytes {
		return ""
	}
	mime := imageMIME(zipPath)
	if mime == "" {
		return ""
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func imageMIME(p string) string {
	switch strings.ToLower(path.Ext(p)) {
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

func findFirst(n *ooxml.Node, local string) *ooxml.Node {
	if n == nil {
		return nil
	}
	if n.Name.Local == local {
		return n
	}
	for _, c := range n.Nodes {
		if got := findFirst(c, local); got != nil {
			return got
		}
	}
	return nil
}

func safeHref(raw string) bool {
	raw = strings.TrimSpace(raw)
	return strings.HasPrefix(raw, "http://") ||
		strings.HasPrefix(raw, "https://") ||
		strings.HasPrefix(raw, "mailto:") ||
		strings.HasPrefix(raw, "#")
}

func safeFont(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' || r == ' ' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
