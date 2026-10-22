// Package markdown implements a small, safe CommonMark/GFM subset.
//
// Raw HTML is always escaped. Link and image destinations are restricted to
// http, https, mailto (links only), and relative URLs. The renderer supports
// headings, paragraphs, lists, blockquotes, fenced code, tables, emphasis,
// inline code, links, images, and task-list checkboxes.
package markdown

import (
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var tableSeparator = regexp.MustCompile(`^:?-{3,}:?$`)

// Render converts Markdown to an HTML fragment.
func Render(src []byte) (string, error) {
	s := strings.ReplaceAll(string(src), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var b strings.Builder
	renderBlocks(&b, strings.Split(s, "\n"))
	return b.String(), nil
}

func renderBlocks(b *strings.Builder, lines []string) {
	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}

		if marker, lang, ok := fence(lines[i]); ok {
			i++
			var code []string
			for i < len(lines) {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), marker) {
					i++
					break
				}
				code = append(code, lines[i])
				i++
			}
			b.WriteString("<pre><code")
			if lang != "" {
				b.WriteString(` class="language-` + html.EscapeString(lang) + `"`)
			}
			b.WriteString(">")
			b.WriteString(html.EscapeString(strings.Join(code, "\n")))
			b.WriteString("</code></pre>\n")
			continue
		}

		if level, text, ok := heading(lines[i]); ok {
			tag := "h" + strconv.Itoa(level)
			b.WriteString("<" + tag + ">" + inline(text) + "</" + tag + ">\n")
			i++
			continue
		}

		if isHorizontalRule(lines[i]) {
			b.WriteString("<hr>\n")
			i++
			continue
		}

		if isBlockquote(lines[i]) {
			var inner []string
			for i < len(lines) && isBlockquote(lines[i]) {
				t := strings.TrimLeft(lines[i], " \t")
				t = strings.TrimPrefix(t, ">")
				t = strings.TrimPrefix(t, " ")
				inner = append(inner, t)
				i++
			}
			b.WriteString("<blockquote>\n")
			renderBlocks(b, inner)
			b.WriteString("</blockquote>\n")
			continue
		}

		if isTableStart(lines, i) {
			i = renderTable(b, lines, i)
			continue
		}

		if _, _, _, ok := listMarker(lines[i], false); ok {
			i = renderList(b, lines, i, false)
			continue
		}
		if _, _, _, ok := listMarker(lines[i], true); ok {
			i = renderList(b, lines, i, true)
			continue
		}

		start := i
		i++
		for i < len(lines) {
			if strings.TrimSpace(lines[i]) == "" || startsBlock(lines, i) {
				break
			}
			i++
		}
		b.WriteString("<p>" + inline(strings.Join(lines[start:i], "\n")) + "</p>\n")
	}
}

func fence(line string) (marker, lang string, ok bool) {
	t := strings.TrimSpace(line)
	if len(t) < 3 || !(strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
		return "", "", false
	}
	marker = t[:3]
	lang = strings.TrimSpace(t[3:])
	if idx := strings.IndexAny(lang, " \t"); idx >= 0 {
		lang = lang[:idx]
	}
	lang = sanitizeLanguage(lang)
	return marker, lang, true
}

func sanitizeLanguage(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '_', r == '-', r == '+', r == '#', r == '.':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func heading(line string) (int, string, bool) {
	t := strings.TrimLeft(line, " ")
	n := 0
	for n < len(t) && n < 6 && t[n] == '#' {
		n++
	}
	if n == 0 || n >= len(t) || t[n] != ' ' {
		return 0, "", false
	}
	return n, strings.TrimSpace(t[n:]), true
}

func isHorizontalRule(line string) bool {
	t := strings.ReplaceAll(strings.TrimSpace(line), " ", "")
	if len(t) < 3 {
		return false
	}
	ch := t[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	for i := 1; i < len(t); i++ {
		if t[i] != ch {
			return false
		}
	}
	return true
}

func isBlockquote(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " \t"), ">")
}

func startsBlock(lines []string, i int) bool {
	if i >= len(lines) || strings.TrimSpace(lines[i]) == "" {
		return true
	}
	if _, _, ok := fence(lines[i]); ok {
		return true
	}
	if _, _, ok := heading(lines[i]); ok {
		return true
	}
	if isHorizontalRule(lines[i]) || isBlockquote(lines[i]) {
		return true
	}
	if _, _, _, ok := listMarker(lines[i], false); ok {
		return true
	}
	if _, _, _, ok := listMarker(lines[i], true); ok {
		return true
	}
	return isTableStart(lines, i)
}

func listMarker(line string, ordered bool) (indent int, content string, start int, ok bool) {
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}
	t := line[indent:]

	if ordered {
		i := 0
		for i < len(t) && t[i] >= '0' && t[i] <= '9' {
			i++
		}
		if i == 0 || i+1 >= len(t) || (t[i] != '.' && t[i] != ')') || t[i+1] != ' ' {
			return 0, "", 0, false
		}
		n, _ := strconv.Atoi(t[:i])
		return indent, strings.TrimSpace(t[i+2:]), n, true
	}

	if len(t) >= 2 && (t[0] == '-' || t[0] == '*' || t[0] == '+') && t[1] == ' ' {
		return indent, strings.TrimSpace(t[2:]), 0, true
	}
	return 0, "", 0, false
}

func renderList(b *strings.Builder, lines []string, i int, ordered bool) int {
	baseIndent, _, first, _ := listMarker(lines[i], ordered)

	tag := "ul"
	if ordered {
		tag = "ol"
	}
	b.WriteString("<" + tag)
	if ordered && first > 1 {
		b.WriteString(` start="` + strconv.Itoa(first) + `"`)
	}
	b.WriteString(">\n")

	for i < len(lines) {
		indent, content, _, ok := listMarker(lines[i], ordered)
		if !ok || indent != baseIndent {
			break
		}
		i++

		item := []string{content}
		for i < len(lines) {
			if strings.TrimSpace(lines[i]) == "" {
				break
			}
			if _, _, _, ok := listMarker(lines[i], ordered); ok {
				break
			}
			if _, _, _, ok := listMarker(lines[i], !ordered); ok {
				break
			}
			if leadingSpaces(lines[i]) <= baseIndent {
				break
			}
			item = append(item, strings.TrimSpace(lines[i]))
			i++
		}

		b.WriteString("<li>")
		text := strings.Join(item, "\n")
		if checked, rest, ok := taskItem(text); ok {
			b.WriteString(`<input type="checkbox" disabled`)
			if checked {
				b.WriteString(` checked`)
			}
			b.WriteString(`> `)
			b.WriteString(inline(rest))
		} else {
			b.WriteString(inline(text))
		}
		b.WriteString("</li>\n")
	}

	b.WriteString("</" + tag + ">\n")
	return i
}

func taskItem(s string) (checked bool, rest string, ok bool) {
	if strings.HasPrefix(s, "[ ] ") {
		return false, s[4:], true
	}
	if strings.HasPrefix(s, "[x] ") || strings.HasPrefix(s, "[X] ") {
		return true, s[4:], true
	}
	return false, "", false
}

func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

func isTableStart(lines []string, i int) bool {
	if i+1 >= len(lines) || !strings.Contains(lines[i], "|") || !strings.Contains(lines[i+1], "|") {
		return false
	}
	header := splitRow(lines[i])
	sep := splitRow(lines[i+1])
	if len(header) == 0 || len(header) != len(sep) {
		return false
	}
	for _, c := range sep {
		if !tableSeparator.MatchString(strings.TrimSpace(c)) {
			return false
		}
	}
	return true
}

func renderTable(b *strings.Builder, lines []string, i int) int {
	header := splitRow(lines[i])
	sep := splitRow(lines[i+1])
	align := make([]string, len(sep))
	for j, c := range sep {
		c = strings.TrimSpace(c)
		switch {
		case strings.HasPrefix(c, ":") && strings.HasSuffix(c, ":"):
			align[j] = "center"
		case strings.HasSuffix(c, ":"):
			align[j] = "right"
		}
	}
	i += 2

	b.WriteString("<table>\n<thead><tr>")
	for j, cell := range header {
		b.WriteString(`<th`)
		if align[j] != "" {
			b.WriteString(` style="text-align:` + align[j] + `"`)
		}
		b.WriteString(`>` + inline(strings.TrimSpace(cell)) + `</th>`)
	}
	b.WriteString("</tr></thead>\n<tbody>\n")

	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" || !strings.Contains(lines[i], "|") {
			break
		}
		row := splitRow(lines[i])
		b.WriteString("<tr>")
		for j := range header {
			b.WriteString(`<td`)
			if align[j] != "" {
				b.WriteString(` style="text-align:` + align[j] + `"`)
			}
			b.WriteString(`>`)
			if j < len(row) {
				b.WriteString(inline(strings.TrimSpace(row[j])))
			}
			b.WriteString("</td>")
		}
		b.WriteString("</tr>\n")
		i++
	}
	b.WriteString("</tbody></table>\n")
	return i
}

func splitRow(line string) []string {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	parts := strings.Split(s, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func inline(s string) string {
	var b strings.Builder
	writeInline(&b, s)
	return b.String()
}

func writeInline(b *strings.Builder, s string) {
	for i := 0; i < len(s); {
		switch s[i] {
		case '\\':
			if i+1 < len(s) && isEscapable(s[i+1]) {
				b.WriteString(html.EscapeString(string(s[i+1])))
				i += 2
			} else {
				b.WriteString(`\`)
				i++
			}

		case '`':
			n := 1
			for i+n < len(s) && s[i+n] == '`' {
				n++
			}
			marker := strings.Repeat("`", n)
			if j := strings.Index(s[i+n:], marker); j >= 0 {
				code := s[i+n : i+n+j]
				b.WriteString("<code>" + html.EscapeString(code) + "</code>")
				i += n + j + n
			} else {
				b.WriteString(marker)
				i += n
			}

		case '!':
			if i+1 < len(s) && s[i+1] == '[' {
				if next, ok := writeLinkOrImage(b, s, i, true); ok {
					i = next
					continue
				}
			}
			b.WriteByte('!')
			i++

		case '[':
			if next, ok := writeLinkOrImage(b, s, i, false); ok {
				i = next
				continue
			}
			b.WriteByte('[')
			i++

		case '<':
			if j := strings.IndexByte(s[i:], '>'); j > 0 {
				raw := s[i+1 : i+j]
				if isAbsoluteHTTP(raw) {
					if u, ok := safeURL(raw, false); ok {
						b.WriteString(`<a href="` + html.EscapeString(u) + `">` +
							html.EscapeString(raw) + `</a>`)
						i += j + 1
						continue
					}
				}
			}
			b.WriteString("&lt;")
			i++

		case '>':
			b.WriteString("&gt;")
			i++

		case '\n':
			b.WriteString("<br>\n")
			i++

		case '*', '_', '~':
			if next, ok := writeEmphasis(b, s, i); ok {
				i = next
				continue
			}
			b.WriteString(html.EscapeString(string(s[i])))
			i++

		default:
			j := i + 1
			for j < len(s) && !isInlineSpecial(s[j]) {
				j++
			}
			b.WriteString(html.EscapeString(s[i:j]))
			i = j
		}
	}
}

func isInlineSpecial(c byte) bool {
	switch c {
	case '\\', '`', '!', '[', '<', '>', '\n', '*', '_', '~':
		return true
	default:
		return false
	}
}

func isEscapable(c byte) bool {
	return strings.ContainsRune("\\`*_{}[]()#+-.!>~|", rune(c))
}

func writeEmphasis(b *strings.Builder, s string, i int) (int, bool) {
	ch := s[i]
	n := 1
	for i+n < len(s) && s[i+n] == ch {
		n++
	}
	if ch == '~' && n != 2 {
		return i, false
	}
	if n > 2 {
		n = 2
	}
	if ch == '_' && i > 0 && isWordByte(s[i-1]) {
		return i, false
	}

	marker := strings.Repeat(string(ch), n)
	bodyStart := i + n
	j := bodyStart
	for {
		k := strings.Index(s[j:], marker)
		if k < 0 {
			return i, false
		}
		pos := j + k
		if pos == bodyStart {
			j = pos + n
			continue
		}
		if ch == '*' && n == 1 {
			// Do not stop at the first half of a ** pair.
			if pos+1 < len(s) && s[pos+1] == '*' {
				j = pos + 2
				continue
			}
		}
		if ch == '_' && pos+n < len(s) && isWordByte(s[pos+n]) {
			j = pos + n
			continue
		}
		inner := s[bodyStart:pos]
		switch {
		case ch == '~' && n == 2:
			b.WriteString("<del>")
			writeInline(b, inner)
			b.WriteString("</del>")
		case n == 2:
			b.WriteString("<strong>")
			writeInline(b, inner)
			b.WriteString("</strong>")
		default:
			b.WriteString("<em>")
			writeInline(b, inner)
			b.WriteString("</em>")
		}
		return pos + n, true
	}
}

func writeLinkOrImage(b *strings.Builder, s string, start int, image bool) (int, bool) {
	labelStart := start + 1
	if image {
		labelStart = start + 2
	}
	closeLabel := strings.IndexByte(s[labelStart:], ']')
	if closeLabel < 0 {
		return start, false
	}
	closeLabel += labelStart
	if closeLabel+1 >= len(s) || s[closeLabel+1] != '(' {
		return start, false
	}

	closeDest, ok := findClosingParen(s, closeLabel+2)
	if !ok {
		return start, false
	}

	label := s[labelStart:closeLabel]
	dest, title := parseDestination(s[closeLabel+2 : closeDest])
	u, ok := safeURL(dest, image)
	if !ok {
		return start, false
	}

	if image {
		b.WriteString(`<img src="` + html.EscapeString(u) + `" alt="` +
			html.EscapeString(label) + `"`)
		if title != "" {
			b.WriteString(` title="` + html.EscapeString(title) + `"`)
		}
		b.WriteString(` loading="lazy">`)
	} else {
		b.WriteString(`<a href="` + html.EscapeString(u) + `"`)
		if title != "" {
			b.WriteString(` title="` + html.EscapeString(title) + `"`)
		}
		b.WriteString(` rel="noopener noreferrer">`)
		writeInline(b, label)
		b.WriteString(`</a>`)
	}
	return closeDest + 1, true
}

func findClosingParen(s string, start int) (int, bool) {
	depth := 0
	var quote byte
	for i := start; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i, true
			}
			depth--
		}
	}
	return 0, false
}

func parseDestination(s string) (dest, title string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if s[0] == '<' {
		if end := strings.IndexByte(s, '>'); end > 0 {
			dest = s[1:end]
			title = strings.TrimSpace(s[end+1:])
		}
	} else {
		if idx := strings.IndexAny(s, " \t"); idx >= 0 {
			dest = s[:idx]
			title = strings.TrimSpace(s[idx+1:])
		} else {
			dest = s
		}
	}
	title = strings.TrimSpace(title)
	if len(title) >= 2 {
		if (title[0] == '"' && title[len(title)-1] == '"') ||
			(title[0] == '\'' && title[len(title)-1] == '\'') ||
			(title[0] == '(' && title[len(title)-1] == ')') {
			title = title[1 : len(title)-1]
		}
	}
	return dest, title
}

func safeURL(raw string, image bool) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	case "mailto":
		if image {
			return "", false
		}
	case "":
		// Relative URL. Reject protocol-relative URLs with an explicit host.
		if strings.HasPrefix(raw, "//") {
			return "", false
		}
	default:
		return "", false
	}
	return u.String(), true
}

func isAbsoluteHTTP(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
