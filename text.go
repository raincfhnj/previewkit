package previewkit

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

// DecodeText converts UTF-8, UTF-16 (with BOM), GBK, or GB18030 bytes to
// UTF-8. Invalid input is returned with replacement characters rather than an
// error.
func DecodeText(src []byte) string {
	switch {
	case bytes.HasPrefix(src, []byte{0xEF, 0xBB, 0xBF}):
		src = src[3:]

	case bytes.HasPrefix(src, []byte{0xFF, 0xFE}):
		src = src[2:]
		if out, err := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).
			NewDecoder().Bytes(src); err == nil {
			return string(out)
		}

	case bytes.HasPrefix(src, []byte{0xFE, 0xFF}):
		src = src[2:]
		if out, err := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).
			NewDecoder().Bytes(src); err == nil {
			return string(out)
		}
	}

	if utf8.Valid(src) {
		return string(src)
	}
	if out, err := simplifiedchinese.GBK.NewDecoder().Bytes(src); err == nil {
		return string(out)
	}
	if out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(src); err == nil {
		return string(out)
	}
	return strings.ToValidUTF8(string(src), "\uFFFD")
}

func isBinary(src []byte) bool {
	if bytes.HasPrefix(src, []byte{0xFF, 0xFE}) ||
		bytes.HasPrefix(src, []byte{0xFE, 0xFF}) {
		return false
	}
	if len(src) > 8192 {
		src = src[:8192]
	}
	return bytes.IndexByte(src, 0) >= 0
}
