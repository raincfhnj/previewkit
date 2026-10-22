# previewkit

一个可被其他 Go 项目直接 `import` 的文件预览库。

`previewkit` 不是 HTTP 服务，也不是 SaaS。它只负责把文件变成浏览器能展示的东西：

- Markdown → 安全的 HTML 片段
- TXT → UTF-8 文本（支持 UTF-8 / UTF-16 / GBK / GB18030）
- XLSX/XLSM → HTML 表格（值、合并单元格、列宽、行高、部分样式）
- PDF / 图片 / 音频 / 视频 → 原文件直通
- DOC/DOCX/XLS/PPT/PPTX 等 → 通过可插拔 `Converter` 转成 PDF

许可证：**Apache-2.0**，允许商用、闭源集成、修改和再分发。

---

## 特性

| 能力 | 说明 |
|---|---|
| 纯 Go 核心 | 不依赖 CGO、不需要 LibreOffice 也能预览 md/txt/xlsx/pdf |
| 可插拔 Office 转换 | 自带 Gotenberg 与本地 LibreOffice 两种 `Converter` |
| 安全默认 | Markdown 经过 bluemonday 清洗；HTML 按文本预览；单元格全部转义 |
| Range 友好 | 返回文件路径，配合 `http.ServeContent` 自动支持分页/拖动进度条 |
| 并发安全 | `Engine` 可并发使用；Office 转换按文件内容去重 |
| 磁盘缓存 | 转换后的 PDF 按 hash 缓存，避免重复转换 |
| 尺寸限制 | 文本大小、Excel 行列数、Office 转换超时均可配置 |
| 宽松许可 | 核心依赖全部为 MIT/BSD/Apache，不含 AGPL/GPL |

---

## 安装

需要 Go 1.26+。

当前 `go.mod` 的 module 名是 `previewkit`，方便本地开发。发布到 GitHub/Gitee 后，把第一行改成你的仓库地址，例如：

```go
module github.com/yourname/previewkit
```

然后其他项目：

```bash
go get github.com/yourname/previewkit
```

本地引用时也可以先用 `replace`：

```
require previewkit v0.0.0
replace previewkit => ../previewkit
```

---

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"os"

	"previewkit"
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
		// res.HTML 是已清洗的 HTML 片段，可直接放进模板
		fmt.Println(string(res.HTML))
	case res.IsText():
		fmt.Println(res.Text)
	case res.IsFile():
		// PDF / 图片 / 音视频 / 转换后的 Office PDF
		fmt.Println(res.ContentType, res.FilePath)
	}
}
```

### 返回结果

```go
type Result struct {
	Kind        Kind
	ContentType string
	Filename    string

	HTML template.HTML // Markdown / Excel
	Text string        // TXT
	FilePath string    // PDF / 图片 / 音视频 / Office 转换结果

	Size      int64
	ModTime   time.Time
	Converted bool
}
```

- `IsHTML()`：Markdown / Excel
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

PDF 建议交给 PDF.js 渲染：

```html
<iframe src="/static/pdfjs/web/viewer.html?file=/api/files/123/raw"></iframe>
```

PDF.js 是 Apache-2.0，可商用；它不是本库的依赖，需要你自己放进前端静态资源。

---

## Office 文档

Office 文档必须经过转换，`previewkit` 不自己实现 Word/Excel 排版。

### 方式一：Gotenberg（推荐）

Gotenberg 是 Go 写的转换服务，MIT 协议，内部使用 LibreOffice。

```bash
docker run --rm -p 3000:3000 gotenberg/gotenberg:8-libreoffice
```

```go
import (
	"previewkit"
	"previewkit/gotenberg"
)

engine := previewkit.New(
	previewkit.WithConverter(gotenberg.New("http://127.0.0.1:3000")),
)
```

中文文档建议构建带字体的镜像：

```dockerfile
FROM gotenberg/gotenberg:8-libreoffice
USER root
COPY fonts/ /usr/share/fonts/truetype/custom/
RUN fc-cache -f
USER gotenberg
```

字体至少放 Noto Sans CJK、文泉驿正黑、Liberation、Carlito、Caladea。

### 方式二：本机 LibreOffice

```go
import "previewkit/libreoffice"

engine := previewkit.New(
	previewkit.WithConverter(libreoffice.New("soffice")),
)
```

`libreoffice.New` 会为每次转换创建独立的 `UserInstallation` 临时目录，避免并发时互相锁住。生产环境建议直接用 Gotenberg，把 `soffice` 隔离在容器里。

### 方式三：自定义 Converter

```go
type MyConverter struct{}

func (MyConverter) ConvertToPDF(ctx context.Context, name string, r io.Reader) ([]byte, error) {
	// 调你的 OnlyOffice / 商业 SDK / 远程转换服务
}
```

```go
engine := previewkit.New(previewkit.WithConverter(MyConverter{}))
```

---

## 支持的格式

| 格式 | 结果 | 依赖 |
|---|---|---|
| `.md` `.markdown` | 安全 HTML | goldmark + bluemonday |
| `.txt` `.log` `.csv` `.json` `.xml` `.yaml` 代码 | 文本 | 纯 Go |
| `.xlsx` `.xlsm` | HTML 表格 | excelize |
| `.pdf` | 原文件 | 无 |
| `.jpg` `.png` `.gif` `.webp` `.svg` 等 | 原文件 | 无 |
| `.mp4` `.webm` `.mov` | 原文件 | 无 |
| `.mp3` `.wav` `.m4a` | 原文件 | 无 |
| `.doc` `.docx` `.xls` `.ppt` `.pptx` `.odt` `.rtf` 等 | 转换后的 PDF | Converter |

`.xls`、`.doc`、`.ppt` 旧格式必须走 Converter，excelize 只支持 OOXML。

---

## XLSX 渲染范围

已支持：

- 单元格值
- 多 sheet + tab
- 合并单元格 `rowspan` / `colspan`
- 列宽、行高
- 粗体、斜体、下划线、字号、字体颜色
- 背景填充色
- 水平/垂直对齐、自动换行

不支持：

- 图表
- 图片、形状
- 条件格式
- 数据透视表
- 公式计算（只显示 Excel 缓存值）
- 复杂数字格式的完整还原

每张表默认最多渲染 5000 行、200 列，可配置：

```go
engine := previewkit.New(
	previewkit.WithMaxExcelRows(2000),
	previewkit.WithMaxExcelCols(80),
)
```

---

## 样式

库只输出 HTML 片段，不注入全局 CSS。建议在宿主页面加：

```css
.pk-tabs { display: flex; gap: 6px; margin: 12px 0; }
.pk-tab { border: 0; background: #f3f4f6; padding: 6px 12px; border-radius: 6px; cursor: pointer; }
.pk-tab.is-active { background: #111827; color: #fff; }

.pk-table { border-collapse: collapse; table-layout: fixed; }
.pk-cell { border: 1px solid #d1d5db; padding: 4px 8px; vertical-align: middle; }
```

sheet 切换：

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
	previewkit.WithMaxExcelRows(5000),
	previewkit.WithMaxExcelCols(200),
)
```

- `WithCacheDir`：转换后的 PDF 缓存目录，默认 `os.UserCacheDir()/previewkit`
- `WithoutCache`：使用临时目录，`Result.Close()` 负责清理
- `WithTimeout`：单次 Office 转换超时，默认 90 秒

缓存 key 包含文件路径、大小、修改时间、Converter 类型和 `cacheFormatVersion`。升级 LibreOffice 或更换字体后，修改 `office.go` 里的 `cacheFormatVersion` 让旧缓存失效。

---

## 安全建议

1. **不要信任扩展名**：上传时校验 magic bytes、大小、病毒扫描。
2. **Markdown 已清洗**：但不要给用户开放任意 HTML 编辑权限。
3. **HTML/SVG**：本库把 `.html` 按纯文本返回；`.svg` 作为图片返回。直接访问 SVG 时请加 `Content-Security-Policy: sandbox`，或放在独立域名。
4. **路径穿越**：不要直接把用户输入的路径传给 `Render`，应该用文件 ID 查数据库得到真实路径。
5. **转换隔离**：Gotenberg/LibreOffice 处理不可信文档时，放在独立容器，限制 CPU/内存/超时，禁止外网。
6. **权限**：预览接口必须校验当前用户是否有权读取该文件。
7. **缓存**：缓存目录不要暴露为静态目录。

---

## 商用与许可证

本库使用 **Apache License 2.0**。

你可以：

- 在商业闭源项目中使用
- 修改源码
- 再分发
- 作为 SaaS 的一部分使用（Apache-2.0 没有 AGPL 的网络 copyleft）

你需要：

- 保留版权声明、LICENSE 和 NOTICE（如有）
- 修改过的文件要标注修改
- 不使用作者商标

### 依赖许可证

| 依赖 | 许可证 | 是否商用友好 |
|---|---|---|
| `github.com/yuin/goldmark` | MIT | 是 |
| `github.com/microcosm-cc/bluemonday` | BSD-3-Clause | 是 |
| `github.com/xuri/excelize/v2` | BSD-3-Clause | 是 |
| `golang.org/x/text` | BSD-3-Clause | 是 |
| `golang.org/x/sync` | BSD-3-Clause | 是 |
| Gotenberg（可选外部服务） | MIT | 是 |
| LibreOffice（可选外部程序） | MPL-2.0 / LGPL-3.0 | 是 |
| PDF.js（可选前端） | Apache-2.0 | 是 |

**本库刻意不引入**：

- MuPDF / `go-fitz`（AGPL）
- Poppler（GPL）
- OnlyOffice DocumentServer（AGPL）

如果以后需要这些能力，请作为可选插件单独发布，避免影响主库的闭源商用。

---

## 限制

- Word/PPT 的版式保真度取决于 LibreOffice/Gotenberg，复杂排版会有差异。
- 不支持 OCR；扫描版 PDF 只能显示图片，不能提取文字。
- 不支持 Office 在线编辑；需要编辑请接 OnlyOffice/Collabora。
- 不支持压缩包、CAD、3D、EPUB 预览；这些可以后续按 `Kind` 扩展。
- 不负责 HTTP 鉴权、上传、存储、病毒扫描；这些由宿主应用负责。

---

## 开发

```bash
go test ./...
go vet ./...
go test -race ./...
```

示例服务：

```bash
go run ./examples/http
# 浏览器访问 http://localhost:8080/preview?path=README.md
```

示例里的 `path` 直接来自 query，仅用于演示；生产环境请改成文件 ID + 权限校验。

---

## 路线图

- [ ] PDF 首页缩略图（`go-pdfium` WASM，MIT）
- [ ] 纯 Go DOCX/PPTX 转换适配器（基于 `omnidoc`，可选子包）
- [ ] 异步转换任务接口
- [ ] S3/MinIO 缓存后端
- [ ] 压缩包目录浏览
- [ ] 水印/页码/下载禁用选项

---

## License

Apache-2.0. See [LICENSE](LICENSE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
