package ooxml

import "testing"

func FuzzParseXML(f *testing.F) {
	f.Add([]byte(`<root><child attr="v">text</child></root>`))
	f.Add([]byte(`<a:root xmlns:a="urn:x"><a:item/></a:root>`))
	f.Add([]byte(`<root>`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseXML(data)
	})
}
