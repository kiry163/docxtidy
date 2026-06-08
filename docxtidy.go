package docxtidy

import (
	"context"
	"io"

	"github.com/kiry163/docxtidy/internal/ooxml"
)

func Extract(ctx context.Context, r io.Reader, opts ExtractOptions) (State, error) {
	rawState, err := ooxml.Extract(ctx, r)
	if err != nil {
		return State{}, err
	}

	return State{
		Document: Document{
			ID:     opts.DocumentID,
			Blocks: blocksFromOOXML(rawState.Blocks),
		},
		Files: packageFilesFromOOXML(rawState.Files),
	}, nil
}

func Write(ctx context.Context, state State, w io.Writer) error {
	return ooxml.Write(ctx, ooxml.State{
		Blocks: blocksToOOXML(state.Document.Blocks),
		Files:  packageFilesToOOXML(state.Files),
	}, w)
}

func packageFilesFromOOXML(files []ooxml.PackageFile) []PackageFile {
	converted := make([]PackageFile, 0, len(files))
	for _, file := range files {
		converted = append(converted, PackageFile{
			Name: file.Name,
			Data: append([]byte(nil), file.Data...),
		})
	}
	return converted
}

func packageFilesToOOXML(files []PackageFile) []ooxml.PackageFile {
	converted := make([]ooxml.PackageFile, 0, len(files))
	for _, file := range files {
		converted = append(converted, ooxml.PackageFile{
			Name: file.Name,
			Data: append([]byte(nil), file.Data...),
		})
	}
	return converted
}

func blocksFromOOXML(blocks []ooxml.Block) []Block {
	converted := make([]Block, 0, len(blocks))
	for _, block := range blocks {
		converted = append(converted, blockFromOOXML(block))
	}
	return converted
}

func blockFromOOXML(block ooxml.Block) Block {
	return Block{
		ID:          block.ID,
		Type:        blockTypeFromOOXML(block.Type),
		Text:        block.Text,
		DisplayText: block.DisplayText,
		XML:         block.XML,
		Numbering:   numberingInfoFromOOXML(block.Numbering),
		Protected:   block.Protected,
	}
}

func blocksToOOXML(blocks []Block) []ooxml.Block {
	converted := make([]ooxml.Block, 0, len(blocks))
	for _, block := range blocks {
		converted = append(converted, blockToOOXML(block))
	}
	return converted
}

func blockToOOXML(block Block) ooxml.Block {
	return ooxml.Block{
		ID:          block.ID,
		Type:        blockTypeToOOXML(block.Type),
		Text:        block.Text,
		DisplayText: block.DisplayText,
		XML:         block.XML,
		Numbering:   numberingInfoToOOXML(block.Numbering),
		Protected:   block.Protected,
	}
}

func blockTypeFromOOXML(blockType ooxml.BlockType) BlockType {
	switch blockType {
	case ooxml.BlockTypeParagraph:
		return BlockTypeParagraph
	case ooxml.BlockTypeTable:
		return BlockTypeTable
	case ooxml.BlockTypeSection:
		return BlockTypeSection
	default:
		return BlockType(blockType)
	}
}

func blockTypeToOOXML(blockType BlockType) ooxml.BlockType {
	switch blockType {
	case BlockTypeParagraph:
		return ooxml.BlockTypeParagraph
	case BlockTypeTable:
		return ooxml.BlockTypeTable
	case BlockTypeSection:
		return ooxml.BlockTypeSection
	default:
		return ooxml.BlockType(blockType)
	}
}

func numberingInfoFromOOXML(info *ooxml.NumberingInfo) *NumberingInfo {
	if info == nil {
		return nil
	}
	return &NumberingInfo{
		Kind:          numberingKindFromOOXML(info.Kind),
		NumID:         info.NumID,
		Level:         info.Level,
		LevelText:     info.LevelText,
		ComputedLabel: info.ComputedLabel,
	}
}

func numberingInfoToOOXML(info *NumberingInfo) *ooxml.NumberingInfo {
	if info == nil {
		return nil
	}
	return &ooxml.NumberingInfo{
		Kind:          numberingKindToOOXML(info.Kind),
		NumID:         info.NumID,
		Level:         info.Level,
		LevelText:     info.LevelText,
		ComputedLabel: info.ComputedLabel,
	}
}

func numberingKindFromOOXML(kind ooxml.NumberingKind) NumberingKind {
	switch kind {
	case ooxml.NumberingKindAuto:
		return NumberingKindAuto
	default:
		return NumberingKind(kind)
	}
}

func numberingKindToOOXML(kind NumberingKind) ooxml.NumberingKind {
	switch kind {
	case NumberingKindAuto:
		return ooxml.NumberingKindAuto
	default:
		return ooxml.NumberingKind(kind)
	}
}
