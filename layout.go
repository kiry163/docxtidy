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
		if err := applyBlockEdit(blockByID, edit); err != nil {
			return Snapshot{}, err
		}
	}
	if layout.AutomaticNumbering == AutomaticNumberingText {
		if err := materializeAutomaticNumbering(blockByID); err != nil {
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
	switch layout.AutomaticNumbering {
	case "", AutomaticNumberingPreserve, AutomaticNumberingText:
	default:
		return fmt.Errorf("unknown automatic numbering policy %q", layout.AutomaticNumbering)
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
		actionCount := 0
		if edit.Replace != nil {
			actionCount++
		}
		if edit.ManualNumbering != nil {
			actionCount++
		}
		if actionCount == 0 {
			return fmt.Errorf("edit for block %s has missing replacement", edit.BlockID)
		}
		if actionCount > 1 {
			return fmt.Errorf("edit for block %s has multiple edit actions", edit.BlockID)
		}
		if edit.Replace != nil && edit.Replace.Old == "" {
			return fmt.Errorf("edit for block %s has empty old text", edit.BlockID)
		}
		if edit.ManualNumbering != nil {
			if block.Type != BlockTypeParagraph {
				return fmt.Errorf("manual numbering edit for block %s requires paragraph block", edit.BlockID)
			}
			if edit.ManualNumbering.Text == "" {
				return fmt.Errorf("edit for block %s has empty manual numbering text", edit.BlockID)
			}
			switch edit.ManualNumbering.Style {
			case "", ManualNumberingStylePlain, ManualNumberingStyleHeading:
			default:
				return fmt.Errorf("edit for block %s has unknown manual numbering style %q", edit.BlockID, edit.ManualNumbering.Style)
			}
		}
	}
	return nil
}

func applyBlockEdit(blockByID map[string]SnapshotBlock, edit Edit) error {
	if edit.Replace != nil {
		return applyBlockTextEdit(blockByID, edit)
	}
	return applyManualNumberingEdit(blockByID, edit)
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

func applyManualNumberingEdit(blockByID map[string]SnapshotBlock, edit Edit) error {
	block := blockByID[edit.BlockID]
	style := string(edit.ManualNumbering.Style)
	updatedXML, err := ooxml.RebuildManualNumberingParagraphXML(block.XML, edit.ManualNumbering.Text, style)
	if err != nil {
		return fmt.Errorf("rebuild manual numbering paragraph %s: %w", edit.BlockID, err)
	}
	block.XML = updatedXML
	block.Text = ooxml.BlockText(updatedXML)
	block.DisplayText = block.Text
	blockByID[edit.BlockID] = block
	return nil
}

func materializeAutomaticNumbering(blockByID map[string]SnapshotBlock) error {
	for blockID, block := range blockByID {
		if block.Type != BlockTypeParagraph || !ooxml.HasParagraphNumbering(block.XML) {
			continue
		}
		prefix, ok := automaticNumberingPrefix(block)
		if !ok {
			continue
		}
		updatedXML, err := ooxml.MaterializeAutomaticNumberingParagraphXML(block.XML, prefix)
		if err != nil {
			return fmt.Errorf("materialize automatic numbering for block %s: %w", blockID, err)
		}
		block.XML = updatedXML
		block.Text = ooxml.BlockText(updatedXML)
		block.DisplayText = block.Text
		blockByID[blockID] = block
	}
	return nil
}

func automaticNumberingPrefix(block SnapshotBlock) (string, bool) {
	if block.DisplayText == "" {
		return "", false
	}
	if block.Text == "" {
		return block.DisplayText, true
	}
	if !strings.HasSuffix(block.DisplayText, block.Text) {
		return "", false
	}
	prefix := strings.TrimSuffix(block.DisplayText, block.Text)
	return prefix, prefix != ""
}

func hasBlockType(blocks []SnapshotBlock, blockType BlockType) bool {
	for _, block := range blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
}
