package previewkit

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestRenderXLSX(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	if err := f.SetCellValue(sheet, "A1", "姓名"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "B1", "年龄"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "A2", "张三"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "B2", 30); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "A3", "合并单元格"); err != nil {
		t.Fatal(err)
	}
	if err := f.MergeCell(sheet, "A3", "B3"); err != nil {
		t.Fatal(err)
	}

	style, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FF0000"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFFF00"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellStyle(sheet, "A1", "A1", style); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := RenderXLSX(path, ExcelOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)

	for _, want := range []string{
		"姓名",
		"张三",
		"30",
		`colspan="2"`,
		"font-weight:600",
		"background-color:#FFFF00",
		`data-pk-sheet="Sheet1"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered HTML does not contain %q", want)
		}
	}
}
