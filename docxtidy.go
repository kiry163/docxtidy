package docxtidy

import (
	"context"
	"io"

	"github.com/kiry163/docxtidy/internal/ooxml"
)

func Extract(ctx context.Context, r io.Reader, opts ExtractOptions) (Snapshot, error) {
	rawState, err := ooxml.Extract(ctx, r)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		Document: DocumentSnapshot{
			ID:     opts.DocumentID,
			Blocks: blocksFromOOXML(rawState.Blocks),
		},
		Package: PackageSnapshot{
			Parts: packagePartsFromOOXML(rawState.Files),
		},
	}, nil
}

func Write(ctx context.Context, snapshot Snapshot, w io.Writer) error {
	return ooxml.Write(ctx, ooxml.State{
		Blocks: blocksToOOXML(snapshot.Document.Blocks),
		Files:  packagePartsToOOXML(snapshot.Package.Parts),
	}, w)
}

func packagePartsFromOOXML(files []ooxml.PackageFile) []PackagePart {
	converted := make([]PackagePart, 0, len(files))
	for _, file := range files {
		converted = append(converted, PackagePart{
			Name: file.Name,
			Data: append([]byte(nil), file.Data...),
		})
	}
	return converted
}

func packagePartsToOOXML(parts []PackagePart) []ooxml.PackageFile {
	converted := make([]ooxml.PackageFile, 0, len(parts))
	for _, part := range parts {
		converted = append(converted, ooxml.PackageFile{
			Name: part.Name,
			Data: append([]byte(nil), part.Data...),
		})
	}
	return converted
}

func blocksFromOOXML(blocks []ooxml.Block) []SnapshotBlock {
	converted := make([]SnapshotBlock, 0, len(blocks))
	for _, block := range blocks {
		converted = append(converted, blockFromOOXML(block))
	}
	return converted
}

func blockFromOOXML(block ooxml.Block) SnapshotBlock {
	return SnapshotBlock{
		ID:          block.ID,
		Type:        blockTypeFromOOXML(block.Type),
		Text:        block.Text,
		DisplayText: block.DisplayText,
		XML:         block.XML,
		Protected:   block.Protected,
	}
}

func blocksToOOXML(blocks []SnapshotBlock) []ooxml.Block {
	converted := make([]ooxml.Block, 0, len(blocks))
	for _, block := range blocks {
		converted = append(converted, blockToOOXML(block))
	}
	return converted
}

func blockToOOXML(block SnapshotBlock) ooxml.Block {
	return ooxml.Block{
		ID:          block.ID,
		Type:        blockTypeToOOXML(block.Type),
		Text:        block.Text,
		DisplayText: block.DisplayText,
		XML:         block.XML,
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
