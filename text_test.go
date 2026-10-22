package previewkit

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func TestDecodeTextUTF8BOM(t *testing.T) {
	src := append([]byte{0xEF, 0xBB, 0xBF}, []byte("你好")...)
	if got := DecodeText(src); got != "你好" {
		t.Fatalf("DecodeText = %q", got)
	}
}

func TestDecodeTextGBK(t *testing.T) {
	// GBK bytes for 中文测试.
	src := []byte{0xD6, 0xD0, 0xCE, 0xC4, 0xB2, 0xE2, 0xCA, 0xD4}
	if got := DecodeText(src); got != "中文测试" {
		t.Fatalf("DecodeText = %q", got)
	}
}

func TestDecodeTextUTF16(t *testing.T) {
	units := utf16.Encode([]rune("hello"))
	body := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(body[i*2:], u)
	}
	src := append([]byte{0xFF, 0xFE}, body...)
	if got := DecodeText(src); got != "hello" {
		t.Fatalf("DecodeText = %q", got)
	}
}
