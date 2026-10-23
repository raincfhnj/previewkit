# previewkit

[![CI](https://github.com/raincfhnj/previewkit/actions/workflows/ci.yml/badge.svg)](https://github.com/raincfhnj/previewkit/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/raincfhnj/previewkit.svg)](https://pkg.go.dev/github.com/raincfhnj/previewkit)

一个可被其他 Go 项目直接 `import` 的文件预览库。

`previewkit` 不是 HTTP 服务，也不是 SaaS。它只负责把文件变成浏览器能展示的东西：

- Markdown → 安全 HTML
- TXT → UTF-8 文本（UTF-8 / UTF-16 / GBK）
- XLSX/XLSM → HTML 表格
- DOCX → HTML（段落、标题、表格、列表、基础样式、图片）
- PPTX → HTML（文本框、基础形状、表格、图片、幻灯片背景）
- PDF / 图片 / 音频 / 视频 → 原文件直通
- DOC/XLS/PPT 等复杂或旧格式 → 通过可插拔 `Converter` 转 PDF

核心模块 **没有第三方 Go 依赖**，`go.mod` 只有 module 和 go 两行。Markdown、XLSX、DOCX、PPTX、文本编码、并发去重全部在 `internal/` 里自己实现。

许可证：**Apache-2.0**，允许商用、闭源集成、修改和再分发。

---

## 特性

| 能力 | 说明 |
|---|---|
| 零第三方核心依赖 | 只用 Go 标准库，无 CGO、无派生仓库、无 AGPL/GPL |
| 自研 Markdown | 安全子集，原始 HTML 一律转义，链接协议白名单，支持嵌套列表 |
| 自研 XLSX | ZIP + XML 解析，支持共享字符串、样式、主题色、合并、列宽、行高 |
| 自研 DOCX | ZIP + XML 解析，支持段落、标题、表格、列表、超链接、内嵌图片 |
| 自研 PPTX | ZIP + XML 解析，支持文本框、形状、表格、图片、背景、占位符布局、主题色、旋转 |
| 自研文本编码 | UTF-8 / UTF-16 / GBK，内置紧凑 GBK 映射表 |
| 自研 singleflight | 同一个 Office 文件并发预览只转换一次 |
| Range 友好 | 返回文件路径，配合 `http.ServeContent` 支持 PDF 分页/视频拖动 |
| 可插拔 Office | 自带 Gotenberg 与本地 LibreOffice 适配器，均只用标准库 |
| 商用友好 | Apache-2.0，核心不含 AGPL/GPL |

---

## 安装

需要 Go 1.22+。

```bash
go get github.com/raincfhnj/previewkit
```

其他项目引用：

```
require github.com/raincfhnj/previewkit v0.5.0
```

本地开发：

```
require github.com/raincfhnj/previewkit v0.0.0
replace github.com/raincfhnj/previewkit => ../previewkit
```

---

## 快速开始

```go
package main

import (
	"context"
	"fmt"

	"github.com/raincfhnj/previewkit"
)

func main() {
	engine := previewkit.New()

	res, err := engine.Render(context.Background(), "docs/readme.md")
	if err != nil {
		panic(err)
	}
	defer res.Close()

	switch {
	case res.IsHTML():
		fmt.Println(string(res.HTML))
	case res.IsText():
		fmt.Println(res.Text)
	case res.IsFile():
		fmt.Println(res.ContentType, res.FilePath)
	}
}
```

DOCX、XLSX 和 PPTX 默认就能预览，不需要任何外部程序。

---

## 返回结果

```go
type Result struct {
	Kind        Kind
	ContentType string
	Filename    string

	HTML template.HTML // Markdown / XLSX / DOCX / PPTX
	Text string        // TXT
	FilePath string    // PDF / 图片 / 音视频 / 转换后的 Office PDF

	Size      int64
	ModTime   time.Time
	Converted bool
}
```

- `IsHTML()`：Markdown / XLSX / DOCX / PPTX
- `IsText()`：TXT
- `IsFile()`：PDF / 图片 / 音频 / 视频 / 转换后的 Office PDF
- `Close()`：释放 `WithoutCache()` 产生的临时文件

---

## 在 HTTP 服务中使用

```go
func previewHandler(engine *previewkit.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := engine.Render(r.Context(), r.URL.Query().Get("path"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Close()

		switch {
		case res.IsHTML():
			w.Header().Set("Content-Type", res.ContentType)
			_, _ = w.Write([]byte(res.HTML))

		case res.IsText():
			w.Header().Set("Content-Type", res.ContentType)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = w.Write([]byte(res.Text))

		default:
			f, err := os.Open(res.FilePath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer f.Close()

			w.Header().Set("Content-Type", res.ContentType)
			w.Header().Set("Content-Disposition",
				"inline; filename*=UTF-8''"+url.PathEscape(res.Filename))
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// 自动处理 Range、If-Modified-Since
			http.ServeContent(w, r, res.Filename, res.ModTime, f)
		}
	}
}
```

PDF 建议交给 PDF.js：

```html
<iframe src="/static/pdfjs/web/viewer.html?file=/api/files/123/raw"></iframe>
```

PDF.js 是 Apache-2.0，可商用；它不是本库依赖，需要你自己放进前端静态资源。

---

## DOCX 自研渲染

`.docx` 不需要 Converter。当前支持：

- 段落、标题（Heading 1–6）
- 粗体、斜体、下划线、删除线
- 字号、字体颜色、高亮/底纹
- 主题色（themeColor，含 tint/shade）
- 左/中/右/两端对齐
- 无序列表、有序列表（基础编号）
- 表格、`gridSpan` 横向合并、`vMerge` 纵向合并
- 超链接（http/https/mailto）
- 内嵌 PNG/JPEG/GIF/WebP/BMP 图片（data URI）
- 多段落、嵌套表格

明确不支持：

- 浮动图片、文本框、艺术字
- 图表、SmartArt、公式（OMML）
- 页眉页脚、批注、修订
- 分栏、精确分页
- `.doc` 旧二进制格式

这是“内容预览”，不是“像素级 Word 排版”。需要高保真时，仍然应该用 LibreOffice/OnlyOffice。

---

## XLSX 自研渲染

`.xlsx` / `.xlsm` 不需要 Converter。当前支持：

- 多 sheet + tab
- 共享字符串、内联字符串
- 数字、布尔、错误值
- 日期/时间（内置日期格式）
- 百分比
- 合并单元格 `rowspan` / `colspan`
- 列宽、行高
- 粗体、斜体、下划线、删除线
- 字号、字体颜色、背景填充色
- 主题色（theme index）
- 单元格边框（thin/medium/thick/dashed/dotted/double）
- 水平/垂直对齐、自动换行

明确不支持：

- 图表
- 图片、形状
- 条件格式
- 数据透视表
- 公式计算（只显示 Excel 缓存值）
- 复杂自定义数字格式

每张表默认最多渲染 2000 行、100 列：

```go
engine := previewkit.New(
	previewkit.WithMaxExcelRows(2000),
	previewkit.WithMaxExcelCols(100),
)
```

---

## PPTX 自研渲染

`.pptx` 不需要 Converter。当前支持：

- 多页幻灯片
- 文本框、占位符布局
- 基础矩形、圆角矩形、椭圆
- 填充色、边框
- 段落对齐、项目符号
- 粗体、斜体、下划线、删除线
- 字号、字体颜色、高亮
- 主题色（accent1/…/dk1/lt1 等）
- 基础旋转、水平/垂直翻转
- 表格（含 `gridSpan` / `rowSpan`）
- PNG/JPEG/GIF/WebP/BMP 图片（data URI）
- 幻灯片背景色

明确不支持：

- 图表、SmartArt
- 动画、切换
- 主题/母版的完整继承
- 组合形状的坐标变换
- 视频、音频
- 备注页

需要像素级 PPT 时，仍然应该用 LibreOffice/OnlyOffice。

---

## 旧格式：Converter

以下格式不自己实现排版，交给外部转换器：

- `.doc`
- `.xls`
- `.ppt`
- `.docm` / `.dotx` / `.dotm`
- `.odt` / `.ods` / `.odp`
- `.rtf` / `.wps` / `.et` / `.dps`

```go
type Converter interface {
	ConvertToPDF(ctx context.Context, filename string, r io.Reader) ([]byte, error)
}
```

### Gotenberg

```bash
docker run --rm -p 3000:3000 gotenberg/gotenberg:8-libreoffice
```

```go
import (
	"github.com/raincfhnj/previewkit"
	"github.com/raincfhnj/previewkit/gotenberg"
)

engine := previewkit.New(
	previewkit.WithConverter(gotenberg.New("http://127.0.0.1:3000")),
)
```

### 本机 LibreOffice

```go
import "github.com/raincfhnj/previewkit/libreoffice"

engine := previewkit.New(
	previewkit.WithConverter(libreoffice.New("soffice")),
)
```

`libreoffice.New` 会为每次转换创建独立的 `UserInstallation` 临时目录，避免并发锁。

### 自定义 Converter

```go
type MyConverter struct{}

func (MyConverter) ConvertToPDF(ctx context.Context, name string, r io.Reader) ([]byte, error) {
	// 调 OnlyOffice / 商业 SDK / 远程转换服务
}
```

---

## 支持格式

| 格式 | 结果 | 实现 |
|---|---|---|
| `.md` `.markdown` | 安全 HTML | 自研 |
| `.txt` `.log` `.csv` `.json` `.xml` `.yaml` 代码 | 文本 | 自研 |
| `.xlsx` `.xlsm` | HTML 表格 | 自研 |
| `.docx` | HTML | 自研 |
| `.pptx` | HTML | 自研 |
| `.pdf` | 原文件 | 直通 |
| `.jpg` `.png` `.gif` `.webp` `.svg` 等 | 原文件 | 直通 |
| `.mp4` `.webm` `.mov` | 原文件 | 直通 |
| `.mp3` `.wav` `.m4a` | 原文件 | 直通 |
| `.doc` `.xls` `.ppt` `.odt` `.rtf` 等 | PDF | Converter |

---

## 样式

库只输出 HTML 片段，不注入全局 CSS。建议在宿主页面加：

```css
.pk-tabs { display: flex; gap: 6px; margin: 12px 0; }
.pk-tab { border: 0; background: #f3f4f6; padding: 6px 12px; border-radius: 6px; cursor: pointer; }
.pk-tab.is-active { background: #111827; color: #fff; }

.pk-table { border-collapse: collapse; table-layout: fixed; }
.pk-cell { border: 1px solid #d1d5db; padding: 4px 8px; vertical-align: middle; }

.pk-docx { max-width: 900px; margin: 24px auto; }
.pk-docx-p { white-space: pre-wrap; margin: 0 0 0.8em; }
.pk-docx-table { border-collapse: collapse; width: 100%; margin: 1em 0; }
.pk-docx-cell { border: 1px solid #d1d5db; padding: 6px 8px; vertical-align: top; }
.pk-docx-marker { display: inline-block; min-width: 1.5em; }
.pk-docx-page-break { border: 0; border-top: 1px dashed #9ca3af; margin: 1.5em 0; }

.pk-pptx { display: flex; flex-direction: column; gap: 24px; align-items: center; padding: 24px; background: #e5e7eb; }
.pk-pptx-slide { position: relative; overflow: hidden; box-shadow: 0 2px 12px rgba(0,0,0,.18); flex: none; }
.pk-pptx-shape { position: absolute; box-sizing: border-box; overflow: hidden; }
.pk-pptx-text { position: absolute; inset: 0; box-sizing: border-box; overflow: hidden; }
.pk-pptx-img { position: absolute; object-fit: contain; }
.pk-pptx-table { width: 100%; height: 100%; border-collapse: collapse; table-layout: fixed; font-size: 14px; }
.pk-pptx-table td { border: 1px solid #9ca3af; padding: 4px 6px; vertical-align: top; overflow: hidden; }
.pk-pptx-page { position: absolute; right: 8px; bottom: 6px; z-index: 10; font-size: 11px; color: #6b7280; }
```

XLSX sheet 切换：

```js
document.querySelectorAll('.pk-tab').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.pk-tab').forEach(b => b.classList.remove('is-active'));
    document.querySelectorAll('.pk-sheet').forEach(s => (s.style.display = 'none'));
    btn.classList.add('is-active');
    document.querySelector(`.pk-sheet[data-pk-sheet="${CSS.escape(btn.dataset.pkSheet)}"]`)
      .style.display = 'block';
  });
});
```

---

## 配置

```go
engine := previewkit.New(
	previewkit.WithConverter(gotenberg.New("http://127.0.0.1:3000")),
	previewkit.WithCacheDir("/var/cache/previewkit"),
	previewkit.WithTimeout(90*time.Second),
	previewkit.WithMaxTextBytes(8<<20),
	previewkit.WithMaxXMLBytes(32<<20),
	previewkit.WithMaxExcelRows(2000),
	previewkit.WithMaxExcelCols(100),
)
```

- `WithCacheDir`：转换后的 PDF 缓存目录，默认 `os.UserCacheDir()/previewkit`
- `WithoutCache`：使用临时目录，`Result.Close()` 负责清理
- `WithTimeout`：单次 Office 转换超时，默认 90 秒

缓存 key 包含文件路径、大小、修改时间、Converter 类型和 `cacheFormatVersion`。升级 LibreOffice 或更换字体后，修改 `office.go` 里的 `cacheFormatVersion` 让旧缓存失效。

---

## 安全建议

1. **不要信任扩展名**：上传时校验 magic bytes、大小、病毒扫描。
2. **Markdown 已安全转义**：原始 HTML 不会执行；链接只允许 http/https/mailto/相对路径。
3. **DOCX/XLSX/PPTX 文本已转义**：不会把用户内容当 HTML 执行。
4. **SVG**：作为图片返回；直接访问时请加 `Content-Security-Policy: sandbox`，或放在独立域名。
5. **路径穿越**：不要直接把用户输入路径传给 `Render`，应该用文件 ID 查数据库得到真实路径。
6. **转换隔离**：Gotenberg/LibreOffice 处理不可信文档时，放在独立容器，限制 CPU/内存/超时，禁止外网。
7. **权限**：预览接口必须校验当前用户是否有权读取该文件。
8. **缓存**：缓存目录不要暴露为静态目录。

---

## 商用与许可证

本库使用 **Apache License 2.0**。

你可以：

- 在商业闭源项目中使用
- 修改源码
- 再分发
- 作为 SaaS 的一部分使用

你需要：

- 保留版权声明、LICENSE 和 NOTICE
- 修改过的文件要标注修改
- 不使用作者商标

### 依赖情况

**核心模块没有任何第三方 Go 依赖。**

`internal/text/gbk.dat` 是根据 Unicode/CP936 映射生成的编码表，不包含第三方源代码。

可选外部工具：

| 工具 | 许可证 | 用途 |
|---|---|---|
| Gotenberg | MIT | 可选 Office 转换服务 |
| LibreOffice | MPL-2.0 / LGPL-3.0 | 可选 Office 转换引擎 |
| PDF.js | Apache-2.0 | 可选前端 PDF 阅读器 |

**本库不包含**：

- MuPDF / `go-fitz`（AGPL）
- Poppler（GPL）
- OnlyOffice DocumentServer（AGPL）
- 任何二次开发/派生仓库代码

---

## 限制

- DOCX/XLSX 是内容预览，不是像素级 Office 排版。
- 不支持 OCR；扫描版 PDF 只能显示，不能提取文字。
- 不支持 Office 在线编辑；需要编辑请接 OnlyOffice/Collabora。
- 不支持压缩包、CAD、3D、EPUB 预览。
- 不负责 HTTP 鉴权、上传、存储、病毒扫描。

---

## 开发

```bash
go test ./...
go vet ./...
go test -race ./...
```

示例服务：

```bash
# 带 Gotenberg（支持旧 Office）
go run ./examples/http

# 零外部依赖（Markdown/TXT/XLSX/DOCX/PPTX/PDF/媒体）
go run ./examples/local
# 浏览器访问 http://localhost:8080/preview?path=README.md
```

示例里的 `path` 直接来自 query，仅用于演示；生产环境请改成文件 ID + 权限校验。

---

## 路线图

- [x] PPTX 自研基础渲染（文本框、形状、表格、图片、背景）
- [x] DOCX `vMerge`
- [ ] DOCX 页眉页脚、脚注、修订
- [ ] XLSX 图表/图片占位
- [ ] PDF 首页缩略图（可选 WASM 模块）
- [ ] 异步转换任务接口
- [ ] S3/MinIO 缓存后端
- [ ] 压缩包目录浏览
- [ ] 水印/页码/下载禁用选项

---

## License

Apache-2.0. See [LICENSE](LICENSE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
