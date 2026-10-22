# Third-Party Notices

`previewkit` is licensed under Apache-2.0.

## Core module

The core module has **no third-party Go dependencies**. It uses only the Go
standard library.

`internal/text/gbk.dat` is a generated CP936/GBK to Unicode mapping table. It
contains encoding data derived from the Unicode character database, not source
code from another project.

## Optional external tools

These are not distributed with this module and are only used if the host
application chooses to configure them:

| Tool | License | Purpose |
|---|---|---|
| Gotenberg | MIT | Optional Office-to-PDF service |
| LibreOffice | MPL-2.0 / LGPL-3.0 | Optional Office conversion engine |
| PDF.js | Apache-2.0 | Optional front-end PDF viewer |

No AGPL or GPL code is included in the `previewkit` module.
