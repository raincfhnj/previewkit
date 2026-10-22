package previewkit

import (
	"mime"
	"path/filepath"
	"strings"
)

// Kind is the preview strategy selected for a file.
type Kind string

const (
	KindMarkdown    Kind = "markdown"
	KindText        Kind = "text"
	KindExcel       Kind = "excel"
	KindDocx        Kind = "docx"
	KindPDF         Kind = "pdf"
	KindOffice      Kind = "office"
	KindImage       Kind = "image"
	KindVideo       Kind = "video"
	KindAudio       Kind = "audio"
	KindUnsupported Kind = "unsupported"
)

// DetectKind selects a preview strategy from the file name.
//
// Detection is intentionally extension-based: OOXML files are ZIP containers
// and magic-byte sniffing alone cannot reliably distinguish DOCX, XLSX, and
// PPTX. Callers that accept untrusted uploads should still validate the
// content, size, and virus status before calling Render.
func DetectKind(name string) Kind {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return KindMarkdown

	case ".txt", ".log", ".csv", ".tsv", ".json", ".xml",
		".yaml", ".yml", ".toml", ".ini", ".conf", ".cfg",
		".go", ".js", ".mjs", ".cjs", ".ts", ".jsx", ".tsx",
		".py", ".rb", ".php", ".java", ".kt", ".kts",
		".c", ".h", ".cc", ".cpp", ".cxx", ".hpp",
		".cs", ".rs", ".swift", ".scala", ".lua", ".pl",
		".sql", ".sh", ".bash", ".zsh", ".ps1", ".bat", ".cmd",
		".html", ".htm", ".css", ".scss", ".less",
		".dockerfile", ".makefile", ".gitignore":
		return KindText

	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return KindExcel

	case ".pdf":
		return KindPDF

	case ".docx":
		return KindDocx

	case ".doc", ".docm", ".dot", ".dotx", ".dotm",
		".xls", ".xlsb", ".xlt", ".xla",
		".ppt", ".pptx", ".pptm", ".pot", ".potx", ".pps", ".ppsx",
		".odt", ".ods", ".odp", ".ott", ".ots", ".otp",
		".rtf", ".wps", ".et", ".dps":
		return KindOffice

	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp",
		".tif", ".tiff", ".ico", ".avif", ".heic", ".heif", ".svg":
		return KindImage

	case ".mp4", ".webm", ".mov", ".m4v", ".ogv", ".mkv":
		return KindVideo

	case ".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac", ".opus":
		return KindAudio

	default:
		return KindUnsupported
	}
}

// ContentTypeFor returns the MIME type used when serving a file or text
// result. HTML and SVG returned by callers must be served with care; see the
// package documentation.
func ContentTypeFor(name string) string {
	ext := strings.ToLower(filepath.Ext(name))

	switch ext {
	case ".md", ".markdown", ".mdown", ".mkd":
		return "text/markdown; charset=utf-8"
	case ".html", ".htm":
		// HTML is previewed as source text, never rendered as active HTML.
		return "text/plain; charset=utf-8"
	case ".txt", ".log", ".gitignore", ".dockerfile", ".makefile":
		return "text/plain; charset=utf-8"
	case ".csv":
		return "text/csv; charset=utf-8"
	case ".tsv":
		return "text/tab-separated-values; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".xml":
		// XML can carry stylesheets; preview it as source text, not active XML.
		return "text/plain; charset=utf-8"
	case ".yaml", ".yml":
		return "application/yaml; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".docx", ".docm", ".dotx", ".dotm":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".pptx", ".pptm", ".potx", ".ppsx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	}

	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
