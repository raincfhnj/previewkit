package previewkit

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderXLSX(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.xlsx")

	writeZip(t, path, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
          xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1"/>
    <sheet name="Sheet2" sheetId="2" r:id="rId2"/>
  </sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
  <Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"xl/sharedStrings.xml": `<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="4" uniqueCount="4">
  <si><t>姓名</t></si>
  <si><t>年龄</t></si>
  <si><t>张三</t></si>
  <si><t>合并单元格</t></si>
</sst>`,
		"xl/styles.xml": `<?xml version="1.0" encoding="UTF-8"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="2">
    <font><sz val="11"/><name val="Calibri"/></font>
    <font><b/><sz val="11"/><color rgb="FFFF0000"/><name val="Calibri"/></font>
  </fonts>
  <fills count="3">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFFFFF00"/><bgColor indexed="64"/></patternFill></fill>
  </fills>
  <cellXfs count="2">
    <xf numFmtId="0" fontId="0" fillId="0"/>
    <xf numFmtId="0" fontId="1" fillId="2" applyFont="1" applyFill="1"/>
  </cellXfs>
</styleSheet>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <cols><col min="1" max="1" width="15" customWidth="1"/></cols>
  <sheetData>
    <row r="1">
      <c r="A1" t="s" s="1"><v>0</v></c>
      <c r="B1" t="s"><v>1</v></c>
    </row>
    <row r="2">
      <c r="A2" t="s"><v>2</v></c>
      <c r="B2"><v>30</v></c>
    </row>
    <row r="3">
      <c r="A3" t="s"><v>3</v></c>
    </row>
  </sheetData>
  <mergeCells count="1"><mergeCell ref="A3:B3"/></mergeCells>
</worksheet>`,
		"xl/worksheets/sheet2.xml": `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="inlineStr"><is><t>第二页</t></is></c></row>
  </sheetData>
</worksheet>`,
	})

	out, err := RenderXLSX(path, ExcelOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	t.Logf("XLSX: %s", s)

	for _, want := range []string{
		"姓名",
		"张三",
		"30",
		"第二页",
		`colspan="2"`,
		"font-weight:600",
		"background-color:#FFFF00",
		`data-pk-sheet="Sheet1"`,
		`width:110px`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered HTML does not contain %q", want)
		}
	}
}
