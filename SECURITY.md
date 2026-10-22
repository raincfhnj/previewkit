# Security

`previewkit` processes documents that may be untrusted. Please treat preview
as a security-sensitive boundary.

## Reporting

Report suspected vulnerabilities privately to the maintainer of the repository
where you obtained this module. Do not open a public issue with a working
exploit or a malicious document attached.

## Threat model

- The library parses Markdown, XLSX, and DOCX in-process. Those parsers are
  pure Go and memory-safe, but parser bugs are still possible.
- Office conversion is performed by an external `Converter`, normally
  Gotenberg or LibreOffice. Those engines are large C/C++ codebases and must be
  isolated from the host.
- The library does not perform authentication, authorization, virus scanning,
  upload validation, or path sandboxing. The host application is responsible
  for those controls.

## Recommended controls

1. Resolve a file ID through your database; never pass a user-supplied
   filesystem path directly to `Render`.
2. Enforce a maximum upload size and a maximum preview size. XML parts are
   limited to 32 MiB, embedded DOCX images to 4 MiB each / 20 MiB total, and
   spreadsheet rendering to 2000 rows x 100 columns by default; lower these
   limits for hostile uploads.
3. Run Gotenberg/LibreOffice in a separate container with CPU and memory
   limits, a read-only root filesystem where possible, and no outbound network.
4. Set conversion timeouts and discard failed outputs.
5. Serve user content from a separate origin, or at minimum set
   `X-Content-Type-Options: nosniff` and a restrictive CSP.
6. Preview `.svg` in an `<img>` tag or with `Content-Security-Policy: sandbox`;
   never navigate directly to an untrusted SVG.
7. Sanitize or escape text before inserting it into HTML. Markdown output is
   already sanitized; plain text is not HTML.
8. Keep Excelize, Goldmark, bluemonday, Go, LibreOffice, and Gotenberg patched.
9. Fuzz the public parsing entry points if you accept hostile documents at
   scale.
