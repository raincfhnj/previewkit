// Package previewkit turns common document formats into browser-previewable
// HTML, plain text, or PDF.
//
// previewkit is a library, not an HTTP service. A host application calls
// Engine.Render and decides how to serve the result:
//
//   - Markdown, XLSX, DOCX, and PPTX produce sanitized HTML fragments.
//   - TXT produces decoded plain text.
//   - PDF, images, audio, and video are passed through unchanged.
//   - Legacy Office formats are converted to PDF by a pluggable Converter.
//
// The core package has no third-party Go dependencies. Markdown, XLSX, DOCX,
// text decoding, and single-flight coordination are implemented in internal
// packages using only the standard library.
package previewkit
