// Package ooxml contains the small ZIP and XML helpers shared by the XLSX and
// DOCX readers. It intentionally depends only on the Go standard library.
package ooxml

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

// Node is a namespace-agnostic XML tree. Element and attribute names are
// compared by their local name, which is sufficient for OOXML.
type Node struct {
	Name  xml.Name
	Attrs []xml.Attr
	Nodes []*Node
	Text  string
}

// ParseXML builds a Node tree from XML bytes.
func ParseXML(data []byte) (*Node, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))

	var root *Node
	var stack []*Node

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			n := &Node{Name: t.Name, Attrs: t.Attr}
			if len(stack) == 0 {
				if root != nil {
					return nil, errors.New("ooxml: multiple root elements")
				}
				root = n
			} else {
				parent := stack[len(stack)-1]
				parent.Nodes = append(parent.Nodes, n)
			}
			stack = append(stack, n)

		case xml.EndElement:
			if len(stack) == 0 {
				return nil, errors.New("ooxml: unbalanced XML")
			}
			stack = stack[:len(stack)-1]

		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(t)
			}
		}
	}

	if root == nil {
		return nil, errors.New("ooxml: empty XML")
	}
	return root, nil
}

// Child returns the first direct child with the given local name.
func (n *Node) Child(local string) *Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Nodes {
		if c.Name.Local == local {
			return c
		}
	}
	return nil
}

// Children returns all direct children with the given local name.
func (n *Node) Children(local string) []*Node {
	if n == nil {
		return nil
	}
	var out []*Node
	for _, c := range n.Nodes {
		if c.Name.Local == local {
			out = append(out, c)
		}
	}
	return out
}

// Attr returns an attribute value by local name.
func (n *Node) Attr(local string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// TextContent concatenates text nodes recursively.
func (n *Node) TextContent() string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	n.appendText(&b)
	return b.String()
}

func (n *Node) appendText(b *strings.Builder) {
	b.WriteString(n.Text)
	for _, c := range n.Nodes {
		c.appendText(b)
	}
}

// ErrEntryTooLarge is returned when a ZIP entry exceeds a configured limit.
var ErrEntryTooLarge = errors.New("ooxml: zip entry too large")

// ReadZipFile reads a ZIP entry by path, tolerating leading slashes and case
// differences.
func ReadZipFile(zr *zip.ReadCloser, name string) ([]byte, error) {
	return ReadZipFileLimited(zr, name, 0)
}

// ReadZipFileLimited reads a ZIP entry and enforces an uncompressed size
// limit. A limit <= 0 means unlimited.
func ReadZipFileLimited(zr *zip.ReadCloser, name string, limit int64) ([]byte, error) {
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	for _, f := range zr.File {
		if !strings.EqualFold(strings.TrimPrefix(f.Name, "/"), name) {
			continue
		}
		if limit > 0 && f.UncompressedSize64 > uint64(limit) {
			return nil, ErrEntryTooLarge
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		if limit <= 0 {
			return io.ReadAll(rc)
		}
		data, err := io.ReadAll(io.LimitReader(rc, limit+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > limit {
			return nil, ErrEntryTooLarge
		}
		return data, nil
	}
	return nil, fs.ErrNotExist
}

// HasZipFile reports whether a ZIP entry exists.
func HasZipFile(zr *zip.ReadCloser, name string) bool {
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	for _, f := range zr.File {
		if strings.EqualFold(strings.TrimPrefix(f.Name, "/"), name) {
			return true
		}
	}
	return false
}

// ResolveTarget resolves an OOXML relationship target against a base
// directory such as "xl" or "word".
func ResolveTarget(base, target string) string {
	if target == "" {
		return ""
	}
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	return path.Clean(path.Join(base, target))
}

// ParseCellRef parses an A1 reference and returns 1-based column and row.
func ParseCellRef(ref string) (col, row int, ok bool) {
	ref = strings.ReplaceAll(strings.TrimSpace(ref), "$", "")
	i := 0
	for i < len(ref) {
		c := ref[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		if c < 'A' || c > 'Z' {
			break
		}
		col = col*26 + int(c-'A'+1)
		i++
	}
	if i == 0 || i >= len(ref) {
		return 0, 0, false
	}
	n, err := strconv.Atoi(ref[i:])
	if err != nil || n <= 0 || col <= 0 {
		return 0, 0, false
	}
	return col, n, true
}

// ColumnName returns the Excel column name for a 1-based column number.
func ColumnName(col int) string {
	if col <= 0 {
		return ""
	}
	var buf [8]byte
	i := len(buf)
	for col > 0 {
		col--
		i--
		buf[i] = byte('A' + col%26)
		col /= 26
	}
	return string(buf[i:])
}

// CellName returns an A1 reference for a 1-based column and row.
func CellName(col, row int) string {
	return ColumnName(col) + strconv.Itoa(row)
}

// ParseInt parses an integer attribute, returning 0 on failure.
func ParseInt(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// ParseFloat parses a float attribute, returning 0 on failure.
func ParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// ParseBool parses common OOXML boolean values.
func ParseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

// HexColor normalizes an OOXML ARGB or RGB color to #RRGGBB.
func HexColor(v string) string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "#")
	if len(v) == 8 {
		v = v[2:]
	}
	if len(v) != 6 {
		return ""
	}
	for i := 0; i < 6; i++ {
		c := v[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return ""
		}
	}
	return "#" + v
}
