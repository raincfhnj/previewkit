package previewkit

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/raincfhnj/previewkit/internal/xlsx"
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
  <Default Extension="png" ContentType="image/png"/>
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
    <font><b/><sz val="11"/><color theme="4"/><name val="Calibri"/></font>
  </fonts>
  <fills count="3">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFFFFF00"/><bgColor indexed="64"/></patternFill></fill>
  </fills>
  <borders count="2">
    <border><left/><right/><top/><bottom/><diagonal/></border>
    <border>
      <left style="thin"><color rgb="FFFF0000"/></left>
      <right style="thin"><color rgb="FFFF0000"/></right>
      <top style="thin"><color rgb="FFFF0000"/></top>
      <bottom style="thin"><color rgb="FFFF0000"/></bottom>
      <diagonal/>
    </border>
  </borders>
  <cellXfs count="2">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>
    <xf numFmtId="0" fontId="1" fillId="2" borderId="1" applyFont="1" applyFill="1" applyBorder="1"/>
  </cellXfs>
</styleSheet>`,
		"xl/theme/theme1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:themeElements>
    <a:clrScheme name="Test">
      <a:dk1><a:srgbClr val="111111"/></a:dk1>
      <a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
      <a:dk2><a:srgbClr val="222222"/></a:dk2>
      <a:lt2><a:srgbClr val="EEEEEE"/></a:lt2>
      <a:accent1><a:srgbClr val="FF00FF"/></a:accent1>
      <a:accent2><a:srgbClr val="00FF00"/></a:accent2>
      <a:accent3><a:srgbClr val="0000FF"/></a:accent3>
      <a:accent4><a:srgbClr val="FFFF00"/></a:accent4>
      <a:accent5><a:srgbClr val="00FFFF"/></a:accent5>
      <a:accent6><a:srgbClr val="FF0000"/></a:accent6>
      <a:hlink><a:srgbClr val="0000EE"/></a:hlink>
      <a:folHlink><a:srgbClr val="551A8B"/></a:folHlink>
    </a:clrScheme>
  </a:themeElements>
</a:theme>`,
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
		"xl/worksheets/_rels/sheet1.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>
</Relationships>`,
		"xl/drawings/drawing1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"
          xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
          xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <xdr:twoCellAnchor editAs="oneCell">
    <xdr:from><xdr:col>1</xdr:col><xdr:colOff>9525</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>9525</xdr:rowOff></xdr:from>
    <xdr:to><xdr:col>3</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>3</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to>
    <xdr:pic>
      <xdr:nvPicPr><xdr:cNvPr id="1" name="Picture 1"/><xdr:cNvPicPr/></xdr:nvPicPr>
      <xdr:blipFill><a:blip r:embed="rId1"/><a:stretch><a:fillRect/></a:stretch></xdr:blipFill>
      <xdr:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="100" cy="100"/></a:xfrm></xdr:spPr>
    </xdr:pic>
    <xdr:clientData/>
  </xdr:twoCellAnchor>
  <xdr:oneCellAnchor>
    <xdr:from><xdr:col>5</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>
    <xdr:ext cx="76200" cy="57150"/>
    <xdr:pic>
      <xdr:nvPicPr><xdr:cNvPr id="2" name="Picture 2"/><xdr:cNvPicPr/></xdr:nvPicPr>
      <xdr:blipFill><a:blip r:embed="rId2"/></xdr:blipFill>
    </xdr:pic>
    <xdr:clientData/>
  </xdr:oneCellAnchor>
  <xdr:absoluteAnchor>
    <xdr:pos x="19050" y="19050"/>
    <xdr:ext cx="9525" cy="9525"/>
    <xdr:pic>
      <xdr:nvPicPr><xdr:cNvPr id="3" name="Picture 3"/><xdr:cNvPicPr/></xdr:nvPicPr>
      <xdr:blipFill><a:blip r:embed="rId3"/></xdr:blipFill>
    </xdr:pic>
    <xdr:clientData/>
  </xdr:absoluteAnchor>
  <xdr:absoluteAnchor>
    <xdr:pos x="0" y="0"/>
    <xdr:ext cx="9525" cy="9525"/>
    <xdr:pic>
      <xdr:nvPicPr><xdr:cNvPr id="4" name="Vector"/><xdr:cNvPicPr/></xdr:nvPicPr>
      <xdr:blipFill><a:blip r:embed="rId4"/></xdr:blipFill>
    </xdr:pic>
    <xdr:clientData/>
  </xdr:absoluteAnchor>
</xdr:wsDr>`,
		"xl/drawings/_rels/drawing1.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image1.png"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image2.png"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image3.png"/>
  <Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image4.svg"/>
</Relationships>`,
		"xl/media/image1.png": "\x89PNG\r\n\x1a\nimage-one",
		"xl/media/image2.png": "\x89PNG\r\n\x1a\nimage-two",
		"xl/media/image3.png": "\x89PNG\r\n\x1a\nimage-three",
		"xl/media/image4.svg": `<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`,
	})

	out, err := RenderXLSX(path, ExcelOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)

	for _, want := range []string{
		"姓名",
		"张三",
		"30",
		"第二页",
		`colspan="2"`,
		"font-weight:600",
		"color:#FF00FF",
		"background-color:#FFFF00",
		"border-left:1px solid #FF0000",
		`data-pk-sheet="Sheet1"`,
		`width:110px`,
		`pk-sheet-canvas`,
		`position:absolute;left:111px;top:21px;width:127px;height:39px`,
		`position:absolute;left:366px;top:20px;width:8px;height:6px`,
		`position:absolute;left:2px;top:2px;width:1px;height:1px`,
		`data:image/png;base64,`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered HTML does not contain %q", want)
		}
	}

	if got := strings.Count(s, `class="pk-sheet-img"`); got != 3 {
		t.Errorf("rendered HTML contains %d images, want 3 (SVG should be skipped)", got)
	}
	if strings.Contains(s, "image/svg") {
		t.Error("rendered HTML inlined an SVG image")
	}
	if got := strings.Count(s, `class="pk-sheet-canvas"`); got != 2 {
		t.Errorf("rendered HTML contains %d sheet canvases, want 2", got)
	}

	limited, err := xlsx.Render(path, xlsx.Options{MaxImageBytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(limited), "pk-sheet-img") {
		t.Error("MaxImageBytes did not skip oversized images")
	}
}
