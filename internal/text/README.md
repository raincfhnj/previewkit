# GBK mapping table

`gbk.dat` is a little-endian `uint16` table covering CP936/GBK double-byte
codes `0x8140` through `0xFEFE` (32,191 entries, 64,382 bytes).

- Index = `code - 0x8140`
- Value = Unicode BMP code point, or `0` for an unmapped/invalid code.

`encoding.go` decodes ASCII directly, maps `0x80` to the euro sign, and falls
back to `U+FFFD` for invalid sequences. Four-byte GB18030 sequences are not
decoded; the table targets the common GBK/GB2312 subset.

The table is generated from the Unicode CP936 mapping and contains only
encoding data, not source code from another project.
