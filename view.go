package docxtidy

import "github.com/kiry163/docxtidy/internal/ooxml"

func ViewOf(state State, opts ViewOptions) View {
	blocks := make([]ViewBlock, 0, len(state.Document.Blocks))
	for index, block := range state.Document.Blocks {
		text := viewBlockText(block, opts.TextMode)
		blocks = append(blocks, ViewBlock{
			ID:        block.ID,
			Index:     index,
			Type:      block.Type,
			Text:      text,
			Protected: block.Protected,
		})
	}

	return View{
		DocumentID: state.Document.ID,
		Blocks:     blocks,
	}
}

func viewBlockText(block Block, mode ViewTextMode) string {
	switch mode {
	case "", ViewTextModeDisplay:
		if block.Type == BlockTypeTable {
			if markdown := ooxml.MarkdownTable(block.XML); markdown != "" {
				return markdown
			}
		}
		if block.DisplayText != "" {
			return block.DisplayText
		}
		return block.Text
	case ViewTextModeSource:
		return block.Text
	default:
		if block.DisplayText != "" {
			return block.DisplayText
		}
		return block.Text
	}
}
