# DocxTidy

DocxTidy 是一个 Go 库，用于将 DOCX 文档提取为可写回的快照，生成适合人工或外部模型阅读的文档轮廓，应用用户组织好的目标布局，并重新写回 DOCX。

库本身是项目的核心。`cmd/docxtidy` CLI 只是围绕公共 API 提供的调试和示例工具。

## 状态

项目目前处于预发布阶段。当前实现适合探索文档块提取和确定性的块重排，但它不是完整的 Word/WPS 渲染引擎。

当前支持范围：

- 提取 `.docx` 包文件和 `word/document.xml` 中的正文块，形成 `Snapshot`。
- 生成用于人工审阅或外部结构识别的紧凑 `Outline`。
- 校验 `Layout` 不会丢失、重复或遗漏块。
- 应用块重排和少量明确文本替换。
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

	snapshot, err := docxtidy.Extract(ctx, input)
	if err != nil {
		panic(err)
	}

	outline := docxtidy.OutlineOf(snapshot)
	_ = outline

	layout := docxtidy.Layout{
		Groups: []docxtidy.Group{
			{BlockIDs: []string{"block-0001"}},
			{BlockIDs: []string{"block-0002"}},
		},
		Edits: []docxtidy.Edit{
			{
				BlockID: "block-0002",
				Replace: &docxtidy.TextReplacement{
					Old: "旧文本",
					New: "新文本",
				},
			},
		},
	}
	_ = layout

	output, err := os.Create("output.docx")
	if err != nil {
		panic(err)
	}
	defer output.Close()

	if err := docxtidy.Write(ctx, snapshot, output); err != nil {
		panic(err)
	}
}
```

`Snapshot` 是完整的写回载体，包含原始包数据和 OOXML 块信息，通常应原样保存。`Outline` 是给人或模型阅读的紧凑表示，不包含写回所需的底层数据。用户或模型可以根据 `Outline` 组织 `Layout`，再调用 `Apply` 得到更新后的 `Snapshot`：

```go
updated, err := docxtidy.Apply(ctx, snapshot, layout)
if err != nil {
	panic(err)
}

if err := docxtidy.Write(ctx, updated, output); err != nil {
	panic(err)
}
```

`Layout` 不包含业务角色语义。库只关心 block id 的新顺序和明确编辑；所有块必须且只能出现一次，遗漏或重复都会报错。

默认行为会保留原 DOCX 自动编号。如果调用方希望把某个自动编号段落改成手写编号，使用 `ManualNumberingEdit`，并传入最终完整文本：

```go
layout.Edits = []docxtidy.Edit{
	{
		BlockID: "block-0040",
		ManualNumbering: &docxtidy.ManualNumberingEdit{
			Text:  "3.4 支撑保障：信息化平台、校企协同机制",
			Style: docxtidy.ManualNumberingStyleHeading,
		},
	},
}
```

该编辑会重建目标段落：移除 DOCX 自动编号属性、移除原段落样式引用和缩进，并写入调用方提供的手写编号文本。`Style` 使用库定义的稳定语义，目前支持 `plain` 和 `heading`，不会要求调用方依赖不同文档中不稳定的 `pStyle` ID。

## CLI

CLI 用于本地调试和示例：

```bash
go run ./cmd/docxtidy extract input.docx --out snapshot.json
go run ./cmd/docxtidy outline snapshot.json --out outline.json
go run ./cmd/docxtidy apply snapshot.json --layout layout.json --out updated-snapshot.json
go run ./cmd/docxtidy write updated-snapshot.json --out output.docx
```

## 测试

```bash
go test ./...
```

测试会在运行时生成最小 DOCX fixture，不需要在仓库中提交 Word 文件。

## 包结构

根包 `docxtidy` 是公共工作流 API。DOCX/OOXML 解析、包重写、展示文本生成、Markdown 表格投影和 XML 文本替换位于 `internal/ooxml`，这些实现细节不会成为公开模块契约的一部分。

## 许可证

MIT
