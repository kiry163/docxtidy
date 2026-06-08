# DocxTidy

DocxTidy 是一个 Go 库，用于将 DOCX 文档提取为块级状态，生成轻量的审阅视图，应用外部提供的结构与变换数据，并重新写回 DOCX。

库本身是项目的核心。`cmd/docxtidy` CLI 只是围绕公共 API 提供的调试和示例工具。

## 状态

项目目前处于预发布阶段。当前实现适合探索文档块提取和确定性的块重排，但它不是完整的 Word/WPS 渲染引擎。

当前支持范围：

- 提取 `.docx` 包文件和 `word/document.xml` 中的正文块。
- 生成用于人工审阅或外部结构识别的紧凑视图。
- 校验变换不会丢失、重复或遗漏块。
- 应用块重排和简单文本编辑。
- 从中间状态重新构建 `.docx`。
- 以 best-effort 方式展示简单编号、表格和图片。

已知限制：

- DOCX 文件会整体读入内存。
- 写回时会重建 ZIP 条目，因此不会保留原始 ZIP 元数据。
- 自动编号支持是 best-effort，不追求完整的 Word/WPS 布局一致性。
- 文本编辑作用于 OOXML 文本节点，复杂文档需要谨慎测试。

## 安装

```bash
go get github.com/kiry163/docxtidy
```

## 库用法

```go
package main

import (
	"context"
	"os"

	"github.com/kiry163/docxtidy"
)

func main() {
	ctx := context.Background()

	input, err := os.Open("input.docx")
	if err != nil {
		panic(err)
	}
	defer input.Close()

	state, err := docxtidy.Extract(ctx, input, docxtidy.ExtractOptions{
		DocumentID: "example",
	})
	if err != nil {
		panic(err)
	}

	view := docxtidy.ViewOf(state, docxtidy.ViewOptions{})
	_ = view

	output, err := os.Create("output.docx")
	if err != nil {
		panic(err)
	}
	defer output.Close()

	if err := docxtidy.Write(ctx, state, output); err != nil {
		panic(err)
	}
}
```

如果需要重排或编辑块，请根据自己的规则、界面或外部模型构造 `Structure` 和 `Transform`，然后在 `Write` 前调用 `Validate` 和 `Apply`。

## CLI

CLI 用于本地调试和示例：

```bash
go run ./cmd/docxtidy extract input.docx --out state.json
go run ./cmd/docxtidy view state.json --out view.json
go run ./cmd/docxtidy apply state.json --structure structure.json --transform transform.json --out new-state.json
go run ./cmd/docxtidy write new-state.json --out output.docx
```

## 测试

```bash
go test ./...
```

测试会在运行时生成最小 DOCX fixture，不需要在仓库中提交 Word 文件。

## 包结构

根包 `docxtidy` 是公共 API。DOCX/OOXML 解析、包重写、展示文本生成、Markdown 表格投影和 XML 文本替换位于 `internal/ooxml`，这些实现细节不会成为公开模块契约的一部分。

## 许可证

MIT
