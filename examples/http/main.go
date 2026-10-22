// Command http demonstrates embedding previewkit in a Go HTTP server.
//
// This example intentionally takes a filesystem path from the query string.
// A real application should resolve a file ID through its database and verify
// that the caller is allowed to read that file.
package main

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"

	"previewkit"
	"previewkit/gotenberg"
)

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title>
<style>
  body{font:15px/1.6 system-ui,-apple-system,"Segoe UI",sans-serif;max-width:960px;margin:40px auto;padding:0 20px;color:#111827}
  .pk-tabs{display:flex;gap:6px;margin:16px 0}
  .pk-tab{border:0;background:#f3f4f6;padding:6px 12px;border-radius:6px;cursor:pointer}
  .pk-tab.is-active{background:#111827;color:#fff}
  .pk-table{border-collapse:collapse;table-layout:fixed}
  .pk-cell{border:1px solid #d1d5db;padding:4px 8px;vertical-align:middle}
  pre{white-space:pre-wrap;word-break:break-word}
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
	engine := previewkit.New(
		previewkit.WithConverter(gotenberg.New(getenv("GOTENBERG_URL", "http://127.0.0.1:3000"))),
		previewkit.WithCacheDir(getenv("PREVIEW_CACHE", "./.preview-cache")),
	)

	http.HandleFunc("/preview", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "missing ?path=", http.StatusBadRequest)
			return
		}

		res, err := engine.Render(r.Context(), path)
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
			serveFile(w, r, res)
		}
	})

	addr := getenv("ADDR", ":8080")
	log.Printf("example listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func serveFile(w http.ResponseWriter, r *http.Request, res *previewkit.Result) {
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

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
