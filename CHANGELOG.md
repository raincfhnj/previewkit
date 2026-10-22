# Changelog

## v0.2.0

- Removed all third-party Go dependencies; the core module now uses only the
  standard library.
- Added self-implemented Markdown rendering with HTML escaping and URL scheme
  allow-listing.
- Added a self-implemented XLSX reader/renderer (shared strings, styles,
  merges, column widths, row heights, multiple sheets).
- Added a self-implemented DOCX reader/renderer (paragraphs, headings, tables,
  lists, character formatting, hyperlinks, inline images).
- Replaced `golang.org/x/text` with a compact embedded GBK mapping table.
- Replaced `golang.org/x/sync/singleflight` with an in-process implementation.
- Added OOXML part and image size limits.
- DOCX no longer requires an external converter.
- Legacy Office formats and PPTX continue to use the pluggable `Converter`
  interface (Gotenberg or local LibreOffice).

## v0.1.0

- Initial library scaffold: Markdown, text, XLSX, PDF passthrough, DOCX via
  converter, Gotenberg and LibreOffice adapters, caching, and tests.
