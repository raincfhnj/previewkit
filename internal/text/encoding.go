// Package text decodes UTF-8, UTF-16, and GBK byte strings without external
// dependencies. The GBK table is generated from the Unicode mapping and stored
// as a compact binary table.
package text

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

//go:embed gbk.dat
var gbkTable []byte

const gbkStart = 0x8140

// Decode converts UTF-8, UTF-16 (with BOM), or GBK bytes to UTF-8.
// Invalid input is returned with replacement characters rather than an error.
func Decode(src []byte) string {
	switch {
	case bytes.HasPrefix(src, []byte{0xEF, 0xBB, 0xBF}):
		src = src[3:]

	case bytes.HasPrefix(src, []byte{0xFF, 0xFE}):
		if out, ok := decodeUTF16(src[2:], binary.LittleEndian); ok {
			return out
		}

	case bytes.HasPrefix(src, []byte{0xFE, 0xFF}):
		if out, ok := decodeUTF16(src[2:], binary.BigEndian); ok {
			return out
		}
	}

	if utf8.Valid(src) {
		return string(src)
	}
	return decodeGBK(src)
}

// IsBinary reports whether data looks like binary rather than text.
// UTF-16 BOMs are treated as text.
func IsBinary(src []byte) bool {
	if bytes.HasPrefix(src, []byte{0xFF, 0xFE}) ||
		bytes.HasPrefix(src, []byte{0xFE, 0xFF}) {
		return false
	}
	if len(src) > 8192 {
		src = src[:8192]
	}
	return bytes.IndexByte(src, 0) >= 0
}

func decodeUTF16(src []byte, order binary.ByteOrder) (string, bool) {
	if len(src)%2 != 0 {
		return "", false
	}
	units := make([]uint16, len(src)/2)
	for i := range units {
		units[i] = order.Uint16(src[i*2:])
	}
	return string(utf16.Decode(units)), true
}

func decodeGBK(src []byte) string {
	var b strings.Builder
	b.Grow(len(src))

	for i := 0; i < len(src); {
		c := src[i]
		if c < 0x80 {
			b.WriteByte(c)
			i++
			continue
		}
		if c == 0x80 {
			b.WriteRune('\u20AC') // CP936 euro sign
			i++
			continue
		}
		if i+1 >= len(src) {
			b.WriteRune(utf8.RuneError)
			break
		}

		next := src[i+1]
		if next < 0x40 || next > 0xFE || next == 0x7F {
			b.WriteRune(utf8.RuneError)
			i++
			continue
		}

		code := int(c)<<8 | int(next)
		idx := code - gbkStart
		if idx >= 0 && idx*2+1 < len(gbkTable) {
			r := rune(binary.LittleEndian.Uint16(gbkTable[idx*2:]))
			if r != 0 {
				b.WriteRune(r)
				i += 2
				continue
			}
		}

		b.WriteRune(utf8.RuneError)
		i += 2
	}
	return b.String()
}
