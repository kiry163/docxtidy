package docxtidy

import "github.com/kiry163/docxtidy/internal/ooxml"

func OutlineOf(snapshot Snapshot) Outline {
	blocks := make([]OutlineBlock, 0, len(snapshot.Document.Blocks))
	for index, block := range snapshot.Document.Blocks {
		blocks = append(blocks, OutlineBlock{
			ID:        block.ID,
			Index:     index,
			Type:      block.Type,
			Text:      outlineBlockText(block),
			Protected: block.Protected,
		})
	}

	return Outline{
		Blocks: blocks,
	}
}

func outlineBlockText(block SnapshotBlock) string {
	if block.Type == BlockTypeTable {
		if markdown := ooxml.MarkdownTable(block.XML); markdown != "" {
			return markdown
		}
	}
	if block.DisplayText != "" {
		return block.DisplayText
	}
	return block.Text
}
