package text

import "testing"

func FuzzDecode(f *testing.F) {
	f.Add([]byte("hello"))
	f.Add([]byte{0xD6, 0xD0, 0xCE, 0xC4})
	f.Add([]byte{0xFF, 0xFE, 'h', 0, 'i', 0})
	f.Add([]byte{0x00, 0x01, 0x02, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		_ = Decode(data)
	})
}
