package previewkit

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderDOCX(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.docx")

	writeZip(t, path, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
  <Override PartName="/word/comments.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"/>
</Types>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdHeader" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
  <Relationship Id="rIdFooter" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>
</Relationships>`,
		"word/header1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>页眉</w:t></w:r></w:p>
</w:hdr>`,
		"word/footer1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>页脚</w:t></w:r></w:p>
</w:ftr>`,
		"word/styles.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
  </w:style>
</w:styles>`,
		"word/theme/theme1.xml": `<?xml version="1.0" encoding="UTF-8"?>
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
		"word/footnotes.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:footnote w:id="1">
    <w:p><w:r><w:footnoteRef/></w:r><w:r><w:t>脚注内容</w:t></w:r></w:p>
  </w:footnote>
</w:footnotes>`,
		"word/comments.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:comment w:id="1" w:author="张三" w:date="2024-01-02T03:04:05Z">
    <w:p><w:r><w:t>批注内容</w:t></w:r></w:p>
  </w:comment>
  <w:comment w:id="2" w:author="李四" w:date="2024-02-03T00:00:00Z">
    <w:p><w:r><w:t>第二条批注</w:t></w:r></w:p>
  </w:comment>
</w:comments>`,
		"word/numbering.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/></w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
</w:numbering>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
            xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading1"/></w:pPr>
      <w:r><w:t>标题</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:rPr><w:b/><w:color w:themeColor="accent1"/></w:rPr><w:t>加粗文字</w:t></w:r>
    </w:p>
    <w:p>
      <w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr>
      <w:r><w:t>列表项</w:t></w:r>
    </w:p>
    <w:p><w:r><w:t>正文</w:t><w:footnoteReference w:id="1"/></w:r></w:p>
    <w:p>
      <w:commentRangeStart w:id="1"/>
      <w:r><w:t>有批注的文字</w:t></w:r>
      <w:r><w:commentReference w:id="1"/></w:r>
      <w:commentRangeEnd w:id="1"/>
      <w:r><w:t>重复引用</w:t><w:commentReference w:id="1"/></w:r>
      <w:r><w:commentReference w:id="2"/></w:r>
    </w:p>
    <w:p>
      <w:ins w:id="10" w:author="张三"><w:r><w:t>插入文字</w:t></w:r></w:ins>
      <w:del w:id="11" w:author="张三"><w:r><w:delText>删除文字</w:delText></w:r></w:del>
      <w:moveTo w:id="12"><w:r><w:t>移入文字</w:t></w:r></w:moveTo>
      <w:moveFrom w:id="13"><w:r><w:delText>移走文字</w:delText></w:r></w:moveFrom>
      <w:r><w:ins><w:t>运行级插入</w:t></w:ins><w:del><w:delText>运行级删除</w:delText></w:del></w:r>
    </w:p>
    <w:tbl>
      <w:tr>
        <w:tc>
          <w:tcPr><w:gridSpan w:val="2"/></w:tcPr>
          <w:p><w:r><w:t>表格</w:t></w:r></w:p>
        </w:tc>
      </w:tr>
    </w:tbl>
    <w:tbl>
      <w:tr>
        <w:tc>
          <w:tcPr><w:vMerge w:val="restart"/></w:tcPr>
          <w:p><w:r><w:t>纵向</w:t></w:r></w:p>
        </w:tc>
        <w:tc><w:p><w:r><w:t>右1</w:t></w:r></w:p></w:tc>
      </w:tr>
      <w:tr>
        <w:tc><w:tcPr><w:vMerge/></w:tcPr><w:p/></w:tc>
        <w:tc><w:p><w:r><w:t>右2</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
    <w:sectPr>
      <w:headerReference w:type="default" r:id="rIdHeader"/>
      <w:footerReference w:type="default" r:id="rIdFooter"/>
    </w:sectPr>
  </w:body>
</w:document>`,
	})

	out, err := RenderDOCX(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)

	for _, want := range []string{
		"<h1",
		"标题",
		"<strong>",
		"color:#FF00FF",
		"• ",
		"列表项",
		`colspan="2"`,
		"表格",
		"纵向",
		"右1",
		"右2",
		`rowspan="2"`,
		`class="pk-docx-header"`,
		"页眉",
		`class="pk-docx-footer"`,
		"页脚",
		`class="pk-docx-fn-ref"`,
		"脚注内容",
		`class="pk-docx-footnotes"`,
		`class="pk-docx-comment-ref"`,
		`href="#comment-1"`,
		"有批注的文字",
		"批注内容",
		`class="pk-docx-comment-meta"`,
		"张三 · 2024-01-02T03:04:05Z",
		"李四 · 2024-02-03T00:00:00Z",
		`class="pk-docx-comments"`,
		`id="comment-1"`,
		`id="comment-2"`,
		"第二条批注",
		`class="pk-docx-ins"`,
		"插入文字",
		"移入文字",
		"运行级插入",
		`class="pk-docx-del"`,
		"删除文字",
		"移走文字",
		"运行级删除",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered HTML does not contain %q", want)
		}
	}

	if got := strings.Count(s, `class="pk-docx-comment-ref"`); got != 3 {
		t.Errorf("comment reference count = %d, want 3", got)
	}
	if got := strings.Count(s, `href="#comment-1"`); got != 2 {
		t.Errorf("comment 1 reference count = %d, want 2 (deduplicated)", got)
	}
}
