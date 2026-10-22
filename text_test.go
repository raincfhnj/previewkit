package previewkit

import (
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

func TestDecodeTextUTF8BOM(t *testing.T) {
	src := append([]byte{0xEF, 0xBB, 0xBF}, []byte("你好")...)
	if got := DecodeText(src); got != "你好" {
		t.Fatalf("DecodeText = %q", got)
	}
}

func TestDecodeTextGBK(t *testing.T) {
	src, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文测试"))
	if err != nil {
		t.Fatal(err)
	}
	if got := DecodeText(src); got != "中文测试" {
		t.Fatalf("DecodeText = %q", got)
	}
}

func TestDecodeTextUTF16(t *testing.T) {
	body, err := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).
		NewEncoder().Bytes([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	src := append([]byte{0xFF, 0xFE}, body...)
	if got := DecodeText(src); got != "hello" {
		t.Fatalf("DecodeText = %q", got)
	}
}
