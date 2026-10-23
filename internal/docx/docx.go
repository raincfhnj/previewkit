// Package docx renders DOCX documents as HTML using only the Go standard
// library.
//
// It is a content preview, not a page-perfect Word renderer: paragraphs,
// headings, tables, basic character formatting, hyperlinks, numbered/bulleted
// lists, and inline images are supported. Floating shapes, charts, equations,
// text boxes, and complex section layout are not.
package docx

import (
	"archive/zip"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/raincfhnj/previewkit/internal/ooxml"
)

// Options controls rendering limits.
type Options struct {
	MaxImageBytes      int
	MaxTotalImageBytes int
	MaxXMLBytes        int64
}

const relNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

type rel struct {
	Target   string
	External bool
}

type charStyle struct {
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
	Size      float64
	Color     string
	Highlight string
}

type styleInfo struct {
	Heading int
	Char    charStyle
	Align   string
}

type docStyles struct {
	raw   map[string]styleInfo
	base  map[string]string
	cache map[string]styleInfo
}

type numLevel struct {
	Format string
	Text   string
	Start  int
}

type docComment struct {
	author string
	date   string
	node   *ooxml.Node
}

type renderer struct {
	styles    *docStyles
	rels      map[string]rel
	numbering map[string]map[int]numLevel
	zr        *zip.ReadCloser
	opts      Options

	listCounters map[string]int
	imageTotal   int
	theme        map[string]string

	footnotes     map[string]*ooxml.Node
	footnoteOrder []string
	footnoteSeen  map[string]bool
	inFootnote    bool
	curFootnote   int

	comments     map[string]*docComment
	commentOrder []string
	commentSeen  map[string]bool
	inComment    bool
}

// Render converts a DOCX document to an HTML fragment.
func Render(path string, opts Options) (template.HTML, error) {
	if opts.MaxImageBytes <= 0 {
		opts.MaxImageBytes = 4 << 20
	}
	if opts.MaxTotalImageBytes <= 0 {
		opts.MaxTotalImageBytes = 20 << 20
	}
	if opts.MaxXMLBytes <= 0 {
		opts.MaxXMLBytes = 32 << 20
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	docXML, err := ooxml.ReadZipFileLimited(zr, "word/document.xml", opts.MaxXMLBytes)
	if err != nil {
		return "", fmt.Errorf("docx: read document: %w", err)
	}
	root, err := ooxml.ParseXML(docXML)
	if err != nil {
		return "", fmt.Errorf("docx: parse document: %w", err)
	}

	rels := map[string]rel{}
	if data, err := ooxml.ReadZipFileLimited(zr, "word/_rels/document.xml.rels", opts.MaxXMLBytes); err == nil {
		if n, err := ooxml.ParseXML(data); err == nil {
			rels = parseRelationships(n)
		}
	}

	theme := ooxml.ReadThemeColors(zr, "word/theme/", opts.MaxXMLBytes)
	footnotes := parseFootnotes(zr, opts.MaxXMLBytes)
	comments := parseComments(zr, opts.MaxXMLBytes)

	var st *docStyles
	if data, err := ooxml.ReadZipFileLimited(zr, "word/styles.xml", opts.MaxXMLBytes); err == nil {
		if n, err := ooxml.ParseXML(data); err == nil {
			st = parseStyles(n, theme)
		}
	}
	if st == nil {
		st = &docStyles{raw: map[string]styleInfo{}, base: map[string]string{}, cache: map[string]styleInfo{}}
	}

	var numbering map[string]map[int]numLevel
	if data, err := ooxml.ReadZipFileLimited(zr, "word/numbering.xml", opts.MaxXMLBytes); err == nil {
		if n, err := ooxml.ParseXML(data); err == nil {
			numbering = parseNumbering(n)
		}
	}

	r := &renderer{
		styles:       st,
		rels:         rels,
		numbering:    numbering,
		zr:           zr,
		opts:         opts,
		listCounters: map[string]int{},
		theme:        theme,
		footnotes:    footnotes,
		footnoteSeen: map[string]bool{},
		comments:     comments,
		commentSeen:  map[string]bool{},
	}

	body := root.Child("body")
	if body == nil {
		return "", fmt.Errorf("docx: document has no body")
	}

	sectPr := findLast(body, "sectPr")

	var b strings.Builder
	b.WriteString(`<div class="pk-docx">`)

	if sectPr != nil {
		if target := partTarget(rels, sectPr, "headerReference"); target != "" {
			partPath := ooxml.ResolveTarget("word", target)
			if ooxml.HasZipFile(zr, partPath) {
				b.WriteString(`<div class="pk-docx-header">`)
				r.renderPart(&b, partPath)
				b.WriteString(`</div>`)
			}
		}
	}

	r.renderBlocks(&b, body)

	if sectPr != nil {
		if target := partTarget(rels, sectPr, "footerReference"); target != "" {
			partPath := ooxml.ResolveTarget("word", target)
			if ooxml.HasZipFile(zr, partPath) {
				b.WriteString(`<div class="pk-docx-footer">`)
				r.renderPart(&b, partPath)
				b.WriteString(`</div>`)
			}
		}
	}

	r.renderFootnotes(&b)
	r.renderComments(&b)
	b.WriteString(`</div>`)
	return template.HTML(b.String()), nil
}

func parseFootnotes(zr *zip.ReadCloser, limit int64) map[string]*ooxml.Node {
	data, err := ooxml.ReadZipFileLimited(zr, "word/footnotes.xml", limit)
	if err != nil {
		return nil
	}
	root, err := ooxml.ParseXML(data)
	if err != nil {
		return nil
	}

	out := map[string]*ooxml.Node{}
	for _, fn := range root.Children("footnote") {
		id := fn.Attr("id")
		if id == "" || id == "-1" || id == "0" {
			continue
		}
		out[id] = fn
	}
	return out
}

func (r *renderer) footnoteRef(n *ooxml.Node) string {
	if r.inFootnote {
		return ""
	}
	id := n.Attr("id")
	fn, ok := r.footnotes[id]
	if !ok || fn == nil {
		return ""
	}

	num := 0
	if !r.footnoteSeen[id] {
		r.footnoteSeen[id] = true
		r.footnoteOrder = append(r.footnoteOrder, id)
		num = len(r.footnoteOrder)
	} else {
		for i, v := range r.footnoteOrder {
			if v == id {
				num = i + 1
				break
			}
		}
	}

	return `<sup class="pk-docx-fn-ref"><a href="#fn-` + html.EscapeString(id) +
		`" id="fnref-` + html.EscapeString(id) + `">[` + strconv.Itoa(num) + `]</a></sup>`
}

func (r *renderer) renderFootnotes(b *strings.Builder) {
	if len(r.footnoteOrder) == 0 {
		return
	}

	r.inFootnote = true
	b.WriteString(`<section class="pk-docx-footnotes"><ol>`)
	for i, id := range r.footnoteOrder {
		r.curFootnote = i + 1
		b.WriteString(`<li id="fn-` + html.EscapeString(id) + `">`)
		r.renderBlocks(b, r.footnotes[id])
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol></section>`)
	r.inFootnote = false
}

func parseComments(zr *zip.ReadCloser, limit int64) map[string]*docComment {
	data, err := ooxml.ReadZipFileLimited(zr, "word/comments.xml", limit)
	if err != nil {
		return nil
	}
	root, err := ooxml.ParseXML(data)
	if err != nil {
		return nil
	}

	out := map[string]*docComment{}
	for _, c := range root.Children("comment") {
		id := c.Attr("id")
		if id == "" {
			continue
		}
		out[id] = &docComment{
			author: c.Attr("author"),
			date:   c.Attr("date"),
			node:   c,
		}
	}
	return out
}

func (r *renderer) commentRef(n *ooxml.Node) string {
	if r.inComment {
		return ""
	}
	id := n.Attr("id")
	c, ok := r.comments[id]
	if !ok || c == nil {
		return ""
	}

	num := 0
	if !r.commentSeen[id] {
		r.commentSeen[id] = true
		r.commentOrder = append(r.commentOrder, id)
		num = len(r.commentOrder)
	} else {
		for i, v := range r.commentOrder {
			if v == id {
				num = i + 1
				break
			}
		}
	}

	return `<sup class="pk-docx-comment-ref"><a href="#comment-` +
		html.EscapeString(id) + `">[` + strconv.Itoa(num) + `]</a></sup>`
}

func (r *renderer) renderComments(b *strings.Builder) {
	if len(r.commentOrder) == 0 {
		return
	}

	r.inComment = true
	b.WriteString(`<section class="pk-docx-comments"><ol>`)
	for _, id := range r.commentOrder {
		c := r.comments[id]
		if c == nil {
			continue
		}
		b.WriteString(`<li id="comment-` + html.EscapeString(id) + `">`)

		meta := c.author
		if meta != "" && c.date != "" {
			meta += " · "
		}
		meta += c.date
		if meta != "" {
			b.WriteString(`<div class="pk-docx-comment-meta">` + html.EscapeString(meta) + `</div>`)
		}

		r.renderBlocks(b, c.node)
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol></section>`)
	r.inComment = false
}

func findLast(n *ooxml.Node, local string) *ooxml.Node {
	if n == nil {
		return nil
	}
	for i := len(n.Nodes) - 1; i >= 0; i-- {
		if n.Nodes[i].Name.Local == local {
			return n.Nodes[i]
		}
		if got := findLast(n.Nodes[i], local); got != nil {
			return got
		}
	}
	return nil
}

func partTarget(rels map[string]rel, sectPr *ooxml.Node, local string) string {
	refs := sectPr.Children(local)
	for _, ref := range refs {
		typ := ref.Attr("type")
		if typ == "" || typ == "default" {
			if rid := relID(ref); rid != "" {
				return rels[rid].Target
			}
		}
	}
	if len(refs) > 0 {
		if rid := relID(refs[0]); rid != "" {
			return rels[rid].Target
		}
	}
	return ""
}

func relID(n *ooxml.Node) string {
	if id := n.AttrNS(relNS, "id"); id != "" {
		return id
	}
	for _, a := range n.Attrs {
		if a.Name.Local == "id" && strings.HasPrefix(a.Value, "rId") {
			return a.Value
		}
	}
	return ""
}

func relsPath(part string) string {
	return path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
}

func readPartRelationships(zr *zip.ReadCloser, part string, limit int64) map[string]rel {
	data, err := ooxml.ReadZipFileLimited(zr, relsPath(part), limit)
	if err != nil {
		return nil
	}
	root, err := ooxml.ParseXML(data)
	if err != nil {
		return nil
	}
	return parseRelationships(root)
}

func (r *renderer) renderPart(b *strings.Builder, partPath string) {
	data, err := ooxml.ReadZipFileLimited(r.zr, partPath, r.opts.MaxXMLBytes)
	if err != nil {
		return
	}
	root, err := ooxml.ParseXML(data)
	if err != nil {
		return
	}

	old := r.rels
	if partRels := readPartRelationships(r.zr, partPath, r.opts.MaxXMLBytes); len(partRels) > 0 {
		merged := make(map[string]rel, len(old)+len(partRels))
		for k, v := range old {
			merged[k] = v
		}
		for k, v := range partRels {
			merged[k] = v
		}
		r.rels = merged
	}
	r.renderBlocks(b, root)
	r.rels = old
}

func parseRelationships(root *ooxml.Node) map[string]rel {
	out := make(map[string]rel)
	for _, n := range root.Children("Relationship") {
		out[n.Attr("Id")] = rel{
			Target:   n.Attr("Target"),
			External: strings.EqualFold(n.Attr("TargetMode"), "External"),
		}
	}
	return out
}

func parseStyles(root *ooxml.Node, theme map[string]string) *docStyles {
	st := &docStyles{
		raw:   map[string]styleInfo{},
		base:  map[string]string{},
		cache: map[string]styleInfo{},
	}

	for _, n := range root.Children("style") {
		id := n.Attr("styleId")
		if id == "" {
			continue
		}

		info := styleInfo{}
		name := ""
		if v := n.Child("name"); v != nil {
			name = v.Attr("val")
		}
		lowerName := strings.ToLower(name)
		lowerID := strings.ToLower(id)
		if strings.HasPrefix(lowerName, "heading ") {
			info.Heading = parseSmallInt(strings.TrimPrefix(lowerName, "heading "))
		} else if strings.HasPrefix(lowerID, "heading") {
			info.Heading = parseSmallInt(strings.TrimPrefix(lowerID, "heading"))
		}
		if info.Heading > 6 {
			info.Heading = 0
		}

		if rPr := n.Child("rPr"); rPr != nil {
			info.Char = parseCharProps(rPr, theme)
		}
		if pPr := n.Child("pPr"); pPr != nil {
			if jc := pPr.Child("jc"); jc != nil {
				info.Align = mapAlign(jc.Attr("val"))
			}
		}

		st.raw[id] = info
		if b := n.Child("basedOn"); b != nil {
			st.base[id] = b.Attr("val")
		}
	}
	return st
}

func parseSmallInt(s string) int {
	var n int
	for i := 0; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

func (s *docStyles) get(id string) styleInfo {
	if id == "" {
		return styleInfo{}
	}
	if v, ok := s.cache[id]; ok {
		return v
	}
	info := s.raw[id]
	if base := s.base[id]; base != "" {
		info = mergeStyle(s.get(base), info)
	}
	s.cache[id] = info
	return info
}

func mergeStyle(base, over styleInfo) styleInfo {
	out := base
	if over.Heading > 0 {
		out.Heading = over.Heading
	}
	out.Char = mergeChar(out.Char, over.Char)
	if over.Align != "" {
		out.Align = over.Align
	}
	return out
}

func mergeChar(base, over charStyle) charStyle {
	out := base
	if over.Bold {
		out.Bold = true
	}
	if over.Italic {
		out.Italic = true
	}
	if over.Underline {
		out.Underline = true
	}
	if over.Strike {
		out.Strike = true
	}
	if over.Size > 0 {
		out.Size = over.Size
	}
	if over.Color != "" {
		out.Color = over.Color
	}
	if over.Highlight != "" {
		out.Highlight = over.Highlight
	}
	return out
}

func parseCharProps(rPr *ooxml.Node, theme map[string]string) charStyle {
	var c charStyle
	c.Bold = onOff(rPr.Child("b"))
	c.Italic = onOff(rPr.Child("i"))
	if u := rPr.Child("u"); u != nil {
		v := u.Attr("val")
		c.Underline = v == "" || v != "none"
	}
	c.Strike = onOff(rPr.Child("strike"))
	if sz := rPr.Child("sz"); sz != nil {
		c.Size = ooxml.ParseFloat(sz.Attr("val")) / 2
	}
	if color := rPr.Child("color"); color != nil {
		c.Color = themeColor(color, theme)
	}
	if hl := rPr.Child("highlight"); hl != nil {
		c.Highlight = highlightColor(hl.Attr("val"))
	}
	if shd := rPr.Child("shd"); shd != nil && c.Highlight == "" {
		c.Highlight = ooxml.HexColor(shd.Attr("fill"))
		if c.Highlight == "" {
			c.Highlight = themeColorByName(
				shd.Attr("themeFill"),
				shd.Attr("themeFillTint"),
				shd.Attr("themeFillShade"),
				theme,
			)
		}
	}
	return c
}

func themeColor(n *ooxml.Node, theme map[string]string) string {
	if c := ooxml.HexColor(n.Attr("val")); c != "" {
		return c
	}
	return themeColorByName(n.Attr("themeColor"), n.Attr("themeTint"), n.Attr("themeShade"), theme)
}

func themeColorByName(name, tint, shade string, theme map[string]string) string {
	if name == "" {
		return ""
	}
	color := theme[themeKey(name)]
	if color == "" {
		return ""
	}
	if shade != "" {
		color = applyShade(color, shade)
	}
	if tint != "" {
		color = applyTint(color, tint)
	}
	return color
}

func themeKey(name string) string {
	switch name {
	case "dark1", "text1":
		return "dk1"
	case "light1", "background1":
		return "lt1"
	case "dark2", "text2":
		return "dk2"
	case "light2", "background2":
		return "lt2"
	case "hyperlink":
		return "hlink"
	case "followedHyperlink":
		return "folHlink"
	default:
		return name
	}
}

func applyShade(color, value string) string {
	r, g, b, ok := parseRGB(color)
	factor, ok2 := parseByte(value)
	if !ok || !ok2 {
		return color
	}
	f := float64(factor) / 255
	return formatRGB(int(float64(r)*f+0.5), int(float64(g)*f+0.5), int(float64(b)*f+0.5))
}

func applyTint(color, value string) string {
	r, g, b, ok := parseRGB(color)
	tint, ok2 := parseByte(value)
	if !ok || !ok2 {
		return color
	}
	t := float64(tint) / 255
	return formatRGB(
		int(float64(r)+(255-float64(r))*t+0.5),
		int(float64(g)+(255-float64(g))*t+0.5),
		int(float64(b)+(255-float64(b))*t+0.5),
	)
}

func parseRGB(color string) (int, int, int, bool) {
	color = strings.TrimPrefix(color, "#")
	if len(color) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(color, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v>>16) & 0xFF, int(v>>8) & 0xFF, int(v) & 0xFF, true
}

func parseByte(value string) (int, bool) {
	v, err := strconv.ParseUint(strings.TrimSpace(value), 16, 8)
	if err != nil {
		return 0, false
	}
	return int(v), true
}

func formatRGB(r, g, b int) string {
	if r < 0 {
		r = 0
	}
	if g < 0 {
		g = 0
	}
	if b < 0 {
		b = 0
	}
	if r > 255 {
		r = 255
	}
	if g > 255 {
		g = 255
	}
	if b > 255 {
		b = 255
	}
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func onOff(n *ooxml.Node) bool {
	if n == nil {
		return false
	}
	v := n.Attr("val")
	return v == "" || ooxml.ParseBool(v)
}

func highlightColor(v string) string {
	switch strings.ToLower(v) {
	case "black":
		return "#000000"
	case "blue":
		return "#0000FF"
	case "cyan":
		return "#00FFFF"
	case "green":
		return "#00FF00"
	case "magenta":
		return "#FF00FF"
	case "red":
		return "#FF0000"
	case "yellow":
		return "#FFFF00"
	case "white":
		return "#FFFFFF"
	case "darkblue":
		return "#000080"
	case "darkcyan":
		return "#008080"
	case "darkgreen":
		return "#008000"
	case "darkmagenta":
		return "#800080"
	case "darkred":
		return "#800000"
	case "darkyellow":
		return "#808000"
	case "darkgray":
		return "#808080"
	case "lightgray":
		return "#D3D3D3"
	default:
		return ""
	}
}

func parseNumbering(root *ooxml.Node) map[string]map[int]numLevel {
	abstracts := map[string]map[int]numLevel{}
	for _, an := range root.Children("abstractNum") {
		id := an.Attr("abstractNumId")
		levels := map[int]numLevel{}
		for _, lvl := range an.Children("lvl") {
			ilvl := ooxml.ParseInt(lvl.Attr("ilvl"))
			nl := numLevel{Start: 1}
			if n := lvl.Child("numFmt"); n != nil {
				nl.Format = n.Attr("val")
			}
			if n := lvl.Child("lvlText"); n != nil {
				nl.Text = n.Attr("val")
			}
			if n := lvl.Child("start"); n != nil {
				nl.Start = ooxml.ParseInt(n.Attr("val"))
			}
			levels[ilvl] = nl
		}
		abstracts[id] = levels
	}

	out := map[string]map[int]numLevel{}
	for _, num := range root.Children("num") {
		numID := num.Attr("numId")
		aid := ""
		if n := num.Child("abstractNumId"); n != nil {
			aid = n.Attr("val")
		}
		out[numID] = abstracts[aid]
	}
	return out
}

func (r *renderer) renderBlocks(b *strings.Builder, parent *ooxml.Node) {
	if parent == nil {
		return
	}
	for _, n := range parent.Nodes {
		switch n.Name.Local {
		case "p":
			r.renderParagraph(b, n)
		case "tbl":
			r.renderTable(b, n)
		case "sdt":
			if content := n.Child("sdtContent"); content != nil {
				r.renderBlocks(b, content)
			}
		}
	}
}

func (r *renderer) renderParagraph(b *strings.Builder, p *ooxml.Node) {
	pPr := p.Child("pPr")
	styleID := ""
	if pPr != nil {
		if ps := pPr.Child("pStyle"); ps != nil {
			styleID = ps.Attr("val")
		}
	}
	st := r.styles.get(styleID)

	align := st.Align
	if pPr != nil {
		if jc := pPr.Child("jc"); jc != nil {
			align = mapAlign(jc.Attr("val"))
		}
	}

	marker := ""
	margin := ""
	if pPr != nil {
		if numPr := pPr.Child("numPr"); numPr != nil {
			numID := ""
			if n := numPr.Child("numId"); n != nil {
				numID = n.Attr("val")
			}
			ilvl := 0
			if n := numPr.Child("ilvl"); n != nil {
				ilvl = ooxml.ParseInt(n.Attr("val"))
			}
			marker = r.listMarker(numID, ilvl)
			margin = fmt.Sprintf("margin-left:%.1fem", float64(ilvl+1)*1.5)
		}
	}
	if marker == "" {
		r.listCounters = map[string]int{}
	}

	var content strings.Builder
	for _, n := range p.Nodes {
		switch n.Name.Local {
		case "r":
			content.WriteString(r.renderRun(n, st.Char))
		case "hyperlink":
			content.WriteString(r.renderHyperlink(n, st.Char))
		case "ins", "moveTo":
			content.WriteString(r.renderTracked(n, st.Char, "ins"))
		case "del", "moveFrom":
			content.WriteString(r.renderTracked(n, st.Char, "del"))
		case "commentReference":
			content.WriteString(r.commentRef(n))
		case "commentRangeStart", "commentRangeEnd":
			// Ranges only mark the commented text; the reference is rendered
			// separately and links to the comment section.
		case "fldSimple":
			for _, c := range n.Children("r") {
				content.WriteString(r.renderRun(c, st.Char))
			}
		case "sdt":
			if sc := n.Child("sdtContent"); sc != nil {
				for _, c := range sc.Children("r") {
					content.WriteString(r.renderRun(c, st.Char))
				}
			}
		}
	}

	style := "white-space:pre-wrap"
	if align != "" {
		style += ";text-align:" + align
	}
	if margin != "" {
		style += ";" + margin
	}

	tag := "p"
	if st.Heading > 0 {
		tag = "h" + strconv.Itoa(st.Heading)
	}

	b.WriteString("<" + tag + ` class="pk-docx-p" style="` + html.EscapeString(style) + `">`)
	if marker != "" {
		b.WriteString(`<span class="pk-docx-marker">` + html.EscapeString(marker) + `</span>`)
	}
	if content.Len() == 0 {
		b.WriteString("<br>")
	} else {
		b.WriteString(content.String())
	}
	b.WriteString("</" + tag + ">\n")
}

func (r *renderer) renderHyperlink(n *ooxml.Node, base charStyle) string {
	var content strings.Builder
	for _, c := range n.Children("r") {
		content.WriteString(r.renderRun(c, base))
	}

	href := ""
	if id := n.Attr("id"); id != "" {
		if rel, ok := r.rels[id]; ok && rel.External {
			href = rel.Target
		}
	}
	if href == "" {
		if anchor := n.Attr("anchor"); anchor != "" {
			href = "#" + anchor
		}
	}
	if href == "" || !safeHref(href) {
		return content.String()
	}
	return `<a href="` + html.EscapeString(href) +
		`" target="_blank" rel="noopener noreferrer">` + content.String() + `</a>`
}

// renderTracked renders the contents of a tracked-change element. tag is "ins"
// for insertions/moveTo and "del" for deletions/moveFrom. Tracked changes may
// contain runs, direct text, or nested tracked-change elements.
func (r *renderer) renderTracked(n *ooxml.Node, base charStyle, tag string) string {
	var content strings.Builder
	for _, c := range n.Nodes {
		switch c.Name.Local {
		case "r":
			content.WriteString(r.renderRun(c, base))
		case "hyperlink":
			content.WriteString(r.renderHyperlink(c, base))
		case "t", "delText":
			content.WriteString(html.EscapeString(c.TextContent()))
		case "ins", "moveTo":
			content.WriteString(r.renderTracked(c, base, "ins"))
		case "del", "moveFrom":
			content.WriteString(r.renderTracked(c, base, "del"))
		case "commentReference":
			content.WriteString(r.commentRef(c))
		}
	}
	if content.Len() == 0 {
		return ""
	}
	return `<` + tag + ` class="pk-docx-` + tag + `">` + content.String() + `</` + tag + `>`
}

func (r *renderer) renderRun(n *ooxml.Node, base charStyle) string {
	cs := base
	if rPr := n.Child("rPr"); rPr != nil {
		if rs := rPr.Child("rStyle"); rs != nil {
			cs = mergeChar(cs, r.styles.get(rs.Attr("val")).Char)
		}
		cs = mergeChar(cs, parseCharProps(rPr, r.theme))
	}

	var content strings.Builder
	for _, c := range n.Nodes {
		switch c.Name.Local {
		case "t", "delText":
			content.WriteString(html.EscapeString(c.TextContent()))
		case "ins", "moveTo":
			content.WriteString(r.renderTracked(c, cs, "ins"))
		case "del", "moveFrom":
			content.WriteString(r.renderTracked(c, cs, "del"))
		case "commentReference":
			content.WriteString(r.commentRef(c))
		case "commentRangeStart", "commentRangeEnd":
			// No-op range markers.
		case "footnoteReference":
			content.WriteString(r.footnoteRef(c))
		case "footnoteRef":
			if r.inFootnote {
				content.WriteString(`<sup class="pk-docx-fn-ref">[` + strconv.Itoa(r.curFootnote) + `]</sup>`)
			}
		case "tab":
			content.WriteString("\t")
		case "br":
			if c.Attr("type") == "page" {
				content.WriteString(`<hr class="pk-docx-page-break">`)
			} else {
				content.WriteString("<br>")
			}
		case "cr":
			content.WriteString("<br>")
		case "noBreakHyphen":
			content.WriteString("-")
		case "softHyphen":
			content.WriteString("&shy;")
		case "drawing", "pict":
			var blips []*ooxml.Node
			collectBlips(c, &blips)
			for _, blip := range blips {
				content.WriteString(r.renderImage(blip.Attr("embed")))
			}
		}
	}

	if content.Len() == 0 {
		return ""
	}

	open, closeTags := wrappers(cs)
	return open + content.String() + closeTags
}

func wrappers(cs charStyle) (string, string) {
	var open, closeTags []string
	if cs.Bold {
		open = append(open, "<strong>")
		closeTags = append(closeTags, "</strong>")
	}
	if cs.Italic {
		open = append(open, "<em>")
		closeTags = append(closeTags, "</em>")
	}
	if cs.Underline {
		open = append(open, "<u>")
		closeTags = append(closeTags, "</u>")
	}
	if cs.Strike {
		open = append(open, "<s>")
		closeTags = append(closeTags, "</s>")
	}

	var css []string
	if cs.Size > 0 {
		css = append(css, fmt.Sprintf("font-size:%.1fpt", cs.Size))
	}
	if cs.Color != "" {
		css = append(css, "color:"+cs.Color)
	}
	if cs.Highlight != "" {
		css = append(css, "background-color:"+cs.Highlight)
	}
	if len(css) > 0 {
		open = append(open, `<span style="`+html.EscapeString(strings.Join(css, ";"))+`">`)
		closeTags = append(closeTags, "</span>")
	}

	// Close in reverse order.
	for i, j := 0, len(closeTags)-1; i < j; i, j = i+1, j-1 {
		closeTags[i], closeTags[j] = closeTags[j], closeTags[i]
	}
	return strings.Join(open, ""), strings.Join(closeTags, "")
}

type tableCell struct {
	node    *ooxml.Node
	colspan int
	rowspan int
	gridCol int
	vmerge  string
	skip    bool
}

type tableRow struct {
	cells []*tableCell
	byCol map[int]*tableCell
}

func (r *renderer) renderTable(b *strings.Builder, tbl *ooxml.Node) {
	rows := parseTableRows(tbl)

	b.WriteString(`<table class="pk-docx-table"><tbody>`)
	for _, row := range rows {
		b.WriteString("<tr>")
		for _, cell := range row.cells {
			if cell.skip {
				continue
			}
			b.WriteString(`<td class="pk-docx-cell"`)
			if cell.colspan > 1 {
				fmt.Fprintf(b, ` colspan="%d"`, cell.colspan)
			}
			if cell.rowspan > 1 {
				fmt.Fprintf(b, ` rowspan="%d"`, cell.rowspan)
			}
			b.WriteString(`>`)
			r.renderBlocks(b, cell.node)
			b.WriteString(`</td>`)
		}
		b.WriteString("</tr>")
	}
	b.WriteString(`</tbody></table>`)
}

func parseTableRows(tbl *ooxml.Node) []tableRow {
	var rows []tableRow

	for _, tr := range tbl.Children("tr") {
		row := tableRow{byCol: map[int]*tableCell{}}
		gridCol := 1

		for _, tc := range tr.Children("tc") {
			colspan, vmerge := 1, ""
			if tcPr := tc.Child("tcPr"); tcPr != nil {
				if gs := tcPr.Child("gridSpan"); gs != nil {
					colspan = ooxml.ParseInt(gs.Attr("val"))
				}
				if vm := tcPr.Child("vMerge"); vm != nil {
					vmerge = vm.Attr("val")
					if vmerge == "" {
						vmerge = "continue"
					}
				}
			}
			if colspan < 1 {
				colspan = 1
			}

			cell := &tableCell{
				node:    tc,
				colspan: colspan,
				gridCol: gridCol,
				vmerge:  vmerge,
			}
			row.cells = append(row.cells, cell)
			for c := gridCol; c < gridCol+colspan; c++ {
				row.byCol[c] = cell
			}
			gridCol += colspan
		}
		rows = append(rows, row)
	}

	// A vMerge restart cell spans across following rows whose corresponding
	// grid column is a vMerge continue cell.
	for i, row := range rows {
		for _, cell := range row.cells {
			if cell.vmerge != "restart" {
				continue
			}
			span := 1
			for j := i + 1; j < len(rows); j++ {
				below := rows[j].byCol[cell.gridCol]
				if below == nil || below.vmerge != "continue" {
					break
				}
				below.skip = true
				span++
			}
			cell.rowspan = span
		}
	}

	return rows
}

func (r *renderer) listMarker(numID string, ilvl int) string {
	if numID == "" {
		return ""
	}
	format := ""
	start := 1
	if levels, ok := r.numbering[numID]; ok {
		if lvl, ok := levels[ilvl]; ok {
			format = lvl.Format
			if lvl.Start > 0 {
				start = lvl.Start
			}
		}
	}

	if format == "decimal" || format == "decimalZero" {
		key := numID + ":" + strconv.Itoa(ilvl)
		r.listCounters[key]++
		return strconv.Itoa(start+r.listCounters[key]-1) + ". "
	}
	return "• "
}

func collectBlips(n *ooxml.Node, out *[]*ooxml.Node) {
	if n.Name.Local == "blip" {
		*out = append(*out, n)
	}
	for _, c := range n.Nodes {
		collectBlips(c, out)
	}
}

func (r *renderer) renderImage(relID string) string {
	if relID == "" {
		return ""
	}
	rel, ok := r.rels[relID]
	if !ok || rel.External {
		return ""
	}
	zipPath := ooxml.ResolveTarget("word", rel.Target)
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
	return `<img src="data:` + mime + `;base64,` +
		base64.StdEncoding.EncodeToString(data) +
		`" alt="" style="max-width:100%;height:auto">`
}

func imageMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
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

func safeHref(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if strings.HasPrefix(raw, "#") {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
		return true
	default:
		return false
	}
}

func mapAlign(v string) string {
	switch strings.ToLower(v) {
	case "left", "center", "right", "justify":
		return strings.ToLower(v)
	case "both", "distribute":
		return "justify"
	default:
		return ""
	}
}
