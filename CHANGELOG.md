# Changelog

## v0.8.0

- DOCX: render footnote references and a footnotes section.
- DOCX: support footnoteRef markers inside footnote content.
- Added footnote fixture coverage.

## v0.7.0

- DOCX: render default section headers and footers.
- DOCX: resolve header/footer relationships, including part-local rels.
- Added header/footer fixture coverage.

## v0.6.0

- PPTX: recursive group shape coordinate transforms.
- PPTX: text, padding, indentation, and spacing scale with group transforms.
- Added a grouped-shape fixture and assertions.

## v0.5.0

- DOCX: resolve `w:themeColor` with tint/shade for text and shading.
- XLSX: render cell borders (left/right/top/bottom) with common styles.
- Markdown: nested list rendering.
- Added DOCX theme-color and XLSX border fixtures.

## v0.4.0

- Added OOXML theme color resolution for PPTX and XLSX:
  - PPTX `schemeClr` (accent1, dk1, lt1, ...).
  - XLSX `theme` indices for fonts and fills.
- Added PPTX theme fixture coverage.
- Added fuzz tests for Markdown, XML parsing, and text decoding.
- CI now runs tests with `-shuffle=on`.

## v0.3.0

- Added DOCX vertical merge (`vMerge`) rowspan support.
- Added PPTX rotation/flip transforms and layout placeholder caching.
- Added a PPTX picture fixture and image pipeline test.
- Added `examples/local`, a zero-converter HTTP example.
- Documented the zero-dependency core and PPTX renderer scope.

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
- Added a self-implemented PPTX reader/renderer (text boxes, basic shapes,
  tables, pictures, backgrounds, placeholder layout).
- Legacy Office formats continue to use the pluggable `Converter` interface
  (Gotenberg or local LibreOffice).

## v0.1.0

- Initial library scaffold: Markdown, text, XLSX, PDF passthrough, DOCX via
  converter, Gotenberg and LibreOffice adapters, caching, and tests.
