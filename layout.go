package docxtidy

import (
	"context"
	"fmt"
	"strings"

	"github.com/kiry163/docxtidy/internal/ooxml"
)

func Validate(ctx context.Context, state State, structure Structure, transform Transform) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	blockByID, err := blockMap(state.Document.Blocks)
	if err != nil {
		return err
	}
	sectionsByRole, err := validateStructure(structure, blockByID)
	if err != nil {
		return err
	}
	return validateTransform(transform, sectionsByRole)
}

func Apply(ctx context.Context, state State, structure Structure, transform Transform) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	blockByID, err := blockMap(state.Document.Blocks)
	if err != nil {
		return State{}, err
	}
	sectionsByRole, err := validateStructure(structure, blockByID)
	if err != nil {
		return State{}, err
	}
	if err := validateTransform(transform, sectionsByRole); err != nil {
		return State{}, err
	}

	for _, edit := range transform.TextEdits {
		if err := applyRoleTextEdit(blockByID, sectionsByRole, edit); err != nil {
			return State{}, err
		}
	}

	rebuiltBlocks := make([]Block, 0, len(state.Document.Blocks))
	for _, role := range transform.Order {
		for _, blockID := range sectionsByRole[role].BlockIDs {
			rebuiltBlocks = append(rebuiltBlocks, blockByID[blockID])
		}
	}
	if len(rebuiltBlocks) != len(state.Document.Blocks) {
		return State{}, fmt.Errorf("transform produced %d blocks, want %d", len(rebuiltBlocks), len(state.Document.Blocks))
	}
	if hasBlockType(state.Document.Blocks, BlockTypeSection) && rebuiltBlocks[len(rebuiltBlocks)-1].Type != BlockTypeSection {
		return State{}, fmt.Errorf("sectPr block must remain last")
	}

	updated := copyState(state)
	updated.Document.Blocks = rebuiltBlocks
	return updated, nil
}

func copyState(state State) State {
	copied := State{
		Document: Document{
			ID:     state.Document.ID,
			Blocks: append([]Block(nil), state.Document.Blocks...),
		},
		Files: append([]PackageFile(nil), state.Files...),
	}
	for i := range copied.Files {
		copied.Files[i].Data = append([]byte(nil), copied.Files[i].Data...)
	}
	return copied
}

func blockMap(blocks []Block) (map[string]Block, error) {
	blockByID := make(map[string]Block, len(blocks))
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

func validateStructure(structure Structure, blockByID map[string]Block) (map[string]Section, error) {
	if len(structure.Sections) == 0 {
		return nil, fmt.Errorf("structure has no sections")
	}

	sectionsByRole := make(map[string]Section, len(structure.Sections))
	seenBlockIDs := map[string]string{}
	for _, section := range structure.Sections {
		if section.Role == "" {
			return nil, fmt.Errorf("structure contains section with empty role")
		}
		if _, exists := sectionsByRole[section.Role]; exists {
			return nil, fmt.Errorf("structure contains duplicate role %s", section.Role)
		}
		if len(section.BlockIDs) == 0 {
			return nil, fmt.Errorf("structure role %s has no block ids", section.Role)
		}
		for _, blockID := range section.BlockIDs {
			if blockID == "" {
				return nil, fmt.Errorf("structure role %s contains empty block id", section.Role)
			}
			if _, exists := blockByID[blockID]; !exists {
				return nil, fmt.Errorf("structure role %s references unknown block %s", section.Role, blockID)
			}
			if previousRole, exists := seenBlockIDs[blockID]; exists {
				return nil, fmt.Errorf("block %s appears in both %s and %s", blockID, previousRole, section.Role)
			}
			seenBlockIDs[blockID] = section.Role
		}
		sectionsByRole[section.Role] = section
	}

	for blockID := range blockByID {
		if _, exists := seenBlockIDs[blockID]; !exists {
			return nil, fmt.Errorf("structure is missing block %s", blockID)
		}
	}
	return sectionsByRole, nil
}

func validateTransform(transform Transform, sectionsByRole map[string]Section) error {
	if len(transform.Order) == 0 {
		return fmt.Errorf("transform order is empty")
	}

	seenRoles := map[string]bool{}
	for _, role := range transform.Order {
		if role == "" {
			return fmt.Errorf("transform order contains empty role")
		}
		if _, exists := sectionsByRole[role]; !exists {
			return fmt.Errorf("transform order references unknown role %s", role)
		}
		if seenRoles[role] {
			return fmt.Errorf("transform order contains duplicate role %s", role)
		}
		seenRoles[role] = true
	}
	for role := range sectionsByRole {
		if !seenRoles[role] {
			return fmt.Errorf("transform order omits role %s", role)
		}
	}
	for _, edit := range transform.TextEdits {
		if edit.Role == "" {
			return fmt.Errorf("text edit contains empty role")
		}
		if edit.Old == "" {
			return fmt.Errorf("text edit for role %s has empty old text", edit.Role)
		}
		if _, exists := sectionsByRole[edit.Role]; !exists {
			return fmt.Errorf("text edit references unknown role %s", edit.Role)
		}
	}
	return nil
}

func applyRoleTextEdit(blockByID map[string]Block, sectionsByRole map[string]Section, edit TextEdit) error {
	section := sectionsByRole[edit.Role]
	for _, blockID := range section.BlockIDs {
		block := blockByID[blockID]
		if !strings.Contains(block.Text, edit.Old) {
			continue
		}
		updatedXML, err := ooxml.ReplaceFirstTextInBlockXML(block.XML, edit.Old, edit.New)
		if err != nil {
			return fmt.Errorf("replace text in %s: %w", blockID, err)
		}
		block.XML = updatedXML
		block.Text = ooxml.BlockText(updatedXML)
		block.DisplayText = strings.Replace(block.DisplayText, edit.Old, edit.New, 1)
		blockByID[blockID] = block
		return nil
	}
	return fmt.Errorf("text %q not found in role %s", edit.Old, edit.Role)
}

func hasBlockType(blocks []Block, blockType BlockType) bool {
	for _, block := range blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
}
