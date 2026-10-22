package previewkit

import "previewkit/internal/text"

// DecodeText converts UTF-8, UTF-16 (with BOM), or GBK bytes to UTF-8.
// Invalid input is returned with replacement characters rather than an error.
func DecodeText(src []byte) string {
	return text.Decode(src)
}

func isBinary(src []byte) bool {
	return text.IsBinary(src)
}
