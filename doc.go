// Package previewkit turns common document formats into browser-previewable
// HTML, plain text, or PDF.
//
// previewkit is a library, not an HTTP service. A host application calls
// Engine.Render and decides how to serve the result:
//
//   - Markdown and XLSX produce sanitized HTML fragments.
//   - TXT produces decoded plain text.
//   - PDF, images, audio, and video are passed through unchanged.
//   - Office documents (DOC/DOCX/XLS/PPT/PPTX and friends) are converted to
//     PDF by a pluggable Converter.
//
// The package itself only depends on permissively licensed, pure-Go modules.
// Office conversion is optional and is provided by the gotenberg and
// libreoffice subpackages, or by any user implementation of Converter.
package previewkit
