package docxtidy

import (
	"context"
	"fmt"
	"strings"

	"github.com/kiry163/docxtidy/internal/ooxml"
)

func Validate(ctx context.Context, snapshot Snapshot, layout Layout) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	blockByID, err := blockMap(snapshot.Document.Blocks)
	if err != nil {
		return err
	}
	return validateLayout(layout, blockByID)
}

func Apply(ctx context.Context, snapshot Snapshot, layout Layout) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	blockByID, err := blockMap(snapshot.Document.Blocks)
	if err != nil {
		return Snapshot{}, err
	}
	if err := validateLayout(layout, blockByID); err != nil {
		return Snapshot{}, err
	}

	for _, edit := range layout.Edits {
		if err := applyBlockTextEdit(blockByID, edit); err != nil {
			return Snapshot{}, err
		}
	}

	rebuiltBlocks := make([]SnapshotBlock, 0, len(snapshot.Document.Blocks))
	for _, group := range layout.Groups {
		for _, blockID := range group.BlockIDs {
			rebuiltBlocks = append(rebuiltBlocks, blockByID[blockID])
		}
	}
	if len(rebuiltBlocks) != len(snapshot.Document.Blocks) {
		return Snapshot{}, fmt.Errorf("layout produced %d blocks, want %d", len(rebuiltBlocks), len(snapshot.Document.Blocks))
	}
	if hasBlockType(snapshot.Document.Blocks, BlockTypeSection) && rebuiltBlocks[len(rebuiltBlocks)-1].Type != BlockTypeSection {
		return Snapshot{}, fmt.Errorf("sectPr block must remain last")
	}

	updated := copySnapshot(snapshot)
	updated.Document.Blocks = rebuiltBlocks
	return updated, nil
}

func copySnapshot(snapshot Snapshot) Snapshot {
	copied := Snapshot{
		Document: DocumentSnapshot{
			Blocks: append([]SnapshotBlock(nil), snapshot.Document.Blocks...),
		},
		Package: PackageSnapshot{
			Parts: append([]PackagePart(nil), snapshot.Package.Parts...),
		},
	}
	for i := range copied.Package.Parts {
		copied.Package.Parts[i].Data = append([]byte(nil), copied.Package.Parts[i].Data...)
	}
	return copied
}

func blockMap(blocks []SnapshotBlock) (map[string]SnapshotBlock, error) {
	blockByID := make(map[string]SnapshotBlock, len(blocks))
	for _, block := range blocks {
		if block.ID == "" {
			return nil, fmt.Errorf("document contains block with empty id")
		}
		if _, exists := blockByID[block.ID]; exists {
			return nil, fmt.Errorf("document contains duplicate block id %s", block.ID)
		}
		blockByID[block.ID] = block
	}
	return blockByID, nil
}

func validateLayout(layout Layout, blockByID map[string]SnapshotBlock) error {
	if len(layout.Groups) == 0 {
		return fmt.Errorf("layout has no groups")
	}

	seenBlockIDs := map[string]bool{}
	for groupIndex, group := range layout.Groups {
		if len(group.BlockIDs) == 0 {
			return fmt.Errorf("layout contains empty group at index %d", groupIndex)
		}
		for _, blockID := range group.BlockIDs {
			if blockID == "" {
				return fmt.Errorf("layout group %d contains empty block id", groupIndex)
			}
			if _, exists := blockByID[blockID]; !exists {
				return fmt.Errorf("layout references unknown block %s", blockID)
			}
			if seenBlockIDs[blockID] {
				return fmt.Errorf("block %s appears more than once in layout", blockID)
			}
			seenBlockIDs[blockID] = true
		}
	}

	for blockID := range blockByID {
		if !seenBlockIDs[blockID] {
			return fmt.Errorf("layout is missing block %s", blockID)
		}
	}

	for _, edit := range layout.Edits {
		if edit.BlockID == "" {
			return fmt.Errorf("edit contains empty block id")
		}
		block, exists := blockByID[edit.BlockID]
		if !exists {
			return fmt.Errorf("edit references unknown block %s", edit.BlockID)
		}
		if block.Protected {
			return fmt.Errorf("cannot edit protected block %s", edit.BlockID)
		}
		if edit.Replace == nil {
			return fmt.Errorf("edit for block %s has missing replacement", edit.BlockID)
		}
		if edit.Replace.Old == "" {
			return fmt.Errorf("edit for block %s has empty old text", edit.BlockID)
		}
	}
	return nil
}

func applyBlockTextEdit(blockByID map[string]SnapshotBlock, edit Edit) error {
	block := blockByID[edit.BlockID]
	if !strings.Contains(block.Text, edit.Replace.Old) {
		return fmt.Errorf("text edit cannot find %q in block %s", edit.Replace.Old, edit.BlockID)
	}
	updatedXML, err := ooxml.ReplaceFirstTextInBlockXML(block.XML, edit.Replace.Old, edit.Replace.New)
	if err != nil {
		return fmt.Errorf("replace text in %s: %w", edit.BlockID, err)
	}
	block.XML = updatedXML
	block.Text = ooxml.BlockText(updatedXML)
	if block.DisplayText != "" {
		block.DisplayText = strings.Replace(block.DisplayText, edit.Replace.Old, edit.Replace.New, 1)
	}
	blockByID[edit.BlockID] = block
	return nil
}

func hasBlockType(blocks []SnapshotBlock, blockType BlockType) bool {
	for _, block := range blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
}
