// Command local demonstrates previewkit with no external converter.
//
// It supports Markdown, TXT, XLSX, DOCX, PPTX, PDF, images, audio, and video.
// Legacy DOC/XLS/PPT files will return ErrConverterRequired.
//
// This example reads a filesystem path from the query string for brevity. A
// real application should resolve a file ID through its database and check
// authorization.
package main

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/raincfhnj/previewkit"
)

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title>
<style>
  body{font:15px/1.6 system-ui,-apple-system,"Segoe UI",sans-serif;max-width:1100px;margin:40px auto;padding:0 20px;color:#111827}
  .pk-tabs{display:flex;gap:6px;margin:12px 0}
  .pk-tab{border:0;background:#f3f4f6;padding:6px 12px;border-radius:6px;cursor:pointer}
  .pk-tab.is-active{background:#111827;color:#fff}
  .pk-table{border-collapse:collapse;table-layout:fixed}
  .pk-cell{border:1px solid #d1d5db;padding:4px 8px;vertical-align:middle}
  .pk-docx-p{white-space:pre-wrap;margin:0 0 .8em}
  .pk-docx-table{border-collapse:collapse;width:100%;margin:1em 0}
  .pk-docx-cell{border:1px solid #d1d5db;padding:6px 8px;vertical-align:top}
  .pk-docx-marker{display:inline-block;min-width:1.5em}
  .pk-docx-page-break{border:0;border-top:1px dashed #9ca3af;margin:1.5em 0}
  .pk-pptx{display:flex;flex-direction:column;gap:24px;align-items:center;padding:24px;background:#e5e7eb}
  .pk-pptx-slide{position:relative;overflow:hidden;box-shadow:0 2px 12px rgba(0,0,0,.18);flex:none}
  .pk-pptx-shape{position:absolute;box-sizing:border-box;overflow:hidden}
  .pk-pptx-text{position:absolute;inset:0;box-sizing:border-box;overflow:hidden}
  .pk-pptx-img{position:absolute;object-fit:contain}
  .pk-pptx-table{width:100%;height:100%;border-collapse:collapse;table-layout:fixed;font-size:14px}
  .pk-pptx-table td{border:1px solid #9ca3af;padding:4px 6px;vertical-align:top;overflow:hidden}
  .pk-pptx-page{position:absolute;right:8px;bottom:6px;z-index:10;font-size:11px;color:#6b7280}
</style>
</head>
<body>
<h1>{{.Title}}</h1>
{{.Body}}
<script>
document.querySelectorAll('.pk-tab').forEach(function (btn) {
  btn.addEventListener('click', function () {
    document.querySelectorAll('.pk-tab').forEach(function (b) { b.classList.remove('is-active'); });
    document.querySelectorAll('.pk-sheet').forEach(function (s) { s.style.display = 'none'; });
    btn.classList.add('is-active');
    document.querySelector('.pk-sheet[data-pk-sheet="' + CSS.escape(btn.dataset.pkSheet) + '"]').style.display = 'block';
  });
});
</script>
</body>
</html>`))

func main() {
	engine := previewkit.New()

	http.HandleFunc("/preview", func(w http.ResponseWriter, r *http.Request) {
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			http.Error(w, "missing ?path=", http.StatusBadRequest)
			return
		}

		res, err := engine.Render(r.Context(), filePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Close()

		switch {
		case res.IsHTML():
			w.Header().Set("Content-Type", res.ContentType)
			_ = page.Execute(w, struct {
				Title string
				Body  template.HTML
			}{res.Filename, res.HTML})

		case res.IsText():
			w.Header().Set("Content-Type", res.ContentType)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = w.Write([]byte(res.Text))

		default:
			f, err := os.Open(res.FilePath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer f.Close()

			w.Header().Set("Content-Type", res.ContentType)
			w.Header().Set("Content-Disposition",
				"inline; filename*=UTF-8''"+url.PathEscape(res.Filename))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			http.ServeContent(w, r, res.Filename, res.ModTime, f)
		}
	})

	addr := ":8080"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}
	log.Printf("local example listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
