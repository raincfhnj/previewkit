package markdown

import "testing"

func FuzzRender(f *testing.F) {
	f.Add("# title\n\n**bold** [link](https://example.com)\n")
	f.Add("| a | b |\n|---|---|\n| 1 | 2 |\n")
	f.Add("<script>alert(1)</script>")
	f.Add("```go\nfmt.Println(\"hi\")\n```")

	f.Fuzz(func(t *testing.T, src string) {
		if _, err := Render([]byte(src)); err != nil {
			t.Fatalf("Render returned an error: %v", err)
		}
	})
}
