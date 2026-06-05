package docxtidy

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

func Validate(ctx context.Context, state DocumentState, structure Structure, layout Layout) error {
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
	return validateLayout(layout, sectionsByRole)
}

func ApplyLayout(ctx context.Context, state DocumentState, structure Structure, layout Layout) (DocumentState, error) {
	if err := ctx.Err(); err != nil {
		return DocumentState{}, err
	}
	blockByID, err := blockMap(state.Document.Blocks)
	if err != nil {
		return DocumentState{}, err
	}
	sectionsByRole, err := validateStructure(structure, blockByID)
	if err != nil {
		return DocumentState{}, err
	}
	if err := validateLayout(layout, sectionsByRole); err != nil {
		return DocumentState{}, err
	}

	for _, replacement := range layout.TextReplacements {
		if err := applyRoleTextReplacement(blockByID, sectionsByRole, replacement); err != nil {
			return DocumentState{}, err
		}
	}

	rebuiltBlocks := make([]Block, 0, len(state.Document.Blocks))
	for _, role := range layout.Order {
		for _, blockID := range sectionsByRole[role].BlockIDs {
			rebuiltBlocks = append(rebuiltBlocks, blockByID[blockID])
		}
	}
	if len(rebuiltBlocks) != len(state.Document.Blocks) {
		return DocumentState{}, fmt.Errorf("layout produced %d blocks, want %d", len(rebuiltBlocks), len(state.Document.Blocks))
	}
	if hasBlockType(state.Document.Blocks, BlockTypeSection) && rebuiltBlocks[len(rebuiltBlocks)-1].Type != BlockTypeSection {
		return DocumentState{}, fmt.Errorf("sectPr block must remain last")
	}

	updated := copyState(state)
	updated.Document.Blocks = rebuiltBlocks
	return updated, nil
}

func copyState(state DocumentState) DocumentState {
	copied := DocumentState{
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

func validateLayout(layout Layout, sectionsByRole map[string]Section) error {
	if len(layout.Order) == 0 {
		return fmt.Errorf("layout order is empty")
	}

	seenRoles := map[string]bool{}
	for _, role := range layout.Order {
		if role == "" {
			return fmt.Errorf("layout order contains empty role")
		}
		if _, exists := sectionsByRole[role]; !exists {
			return fmt.Errorf("layout order references unknown role %s", role)
		}
		if seenRoles[role] {
			return fmt.Errorf("layout order contains duplicate role %s", role)
		}
		seenRoles[role] = true
	}
	for role := range sectionsByRole {
		if !seenRoles[role] {
			return fmt.Errorf("layout order omits role %s", role)
		}
	}
	for _, replacement := range layout.TextReplacements {
		if replacement.Role == "" {
			return fmt.Errorf("text replacement contains empty role")
		}
		if replacement.Old == "" {
			return fmt.Errorf("text replacement for role %s has empty old text", replacement.Role)
		}
		if _, exists := sectionsByRole[replacement.Role]; !exists {
			return fmt.Errorf("text replacement references unknown role %s", replacement.Role)
		}
	}
	return nil
}

func applyRoleTextReplacement(blockByID map[string]Block, sectionsByRole map[string]Section, replacement TextReplacement) error {
	section := sectionsByRole[replacement.Role]
	for _, blockID := range section.BlockIDs {
		block := blockByID[blockID]
		if !strings.Contains(block.Text, replacement.Old) {
			continue
		}
		updatedXML, err := replaceFirstTextInBlockXML(block.XML, replacement.Old, replacement.New)
		if err != nil {
			return fmt.Errorf("replace text in %s: %w", blockID, err)
		}
		block.XML = updatedXML
		block.Text = blockText([]byte(updatedXML))
		block.DisplayText = strings.Replace(block.DisplayText, replacement.Old, replacement.New, 1)
		blockByID[blockID] = block
		return nil
	}
	return fmt.Errorf("text %q not found in role %s", replacement.Old, replacement.Role)
}

func hasBlockType(blocks []Block, blockType BlockType) bool {
	for _, block := range blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
}

type textRange struct {
	start int
	end   int
	text  string
}

func replaceFirstTextInBlockXML(blockXML string, oldText string, newText string) (string, error) {
	ranges, fullText, err := textRangesInBlockXML(blockXML)
	if err != nil {
		return "", err
	}
	if len(ranges) == 0 {
		return "", fmt.Errorf("block has no text nodes")
	}

	replaceStart := strings.Index(fullText, oldText)
	if replaceStart == -1 {
		return "", fmt.Errorf("text %q not found", oldText)
	}
	replaceEnd := replaceStart + len(oldText)

	affectedStart := -1
	affectedEnd := -1
	cursor := 0
	rangeStarts := make([]int, len(ranges))
	rangeEnds := make([]int, len(ranges))
	for i, textRange := range ranges {
		rangeStarts[i] = cursor
		cursor += len(textRange.text)
		rangeEnds[i] = cursor
		if rangeEnds[i] > replaceStart && rangeStarts[i] < replaceEnd {
			if affectedStart == -1 {
				affectedStart = i
			}
			affectedEnd = i
		}
	}
	if affectedStart == -1 {
		return "", fmt.Errorf("text %q did not overlap text nodes", oldText)
	}

	replacements := make(map[int]string)
	first := ranges[affectedStart]
	firstPrefix := first.text[:replaceStart-rangeStarts[affectedStart]]
	if affectedStart == affectedEnd {
		firstSuffix := first.text[replaceEnd-rangeStarts[affectedStart]:]
		replacements[affectedStart] = firstPrefix + newText + firstSuffix
	} else {
		last := ranges[affectedEnd]
		lastSuffix := last.text[replaceEnd-rangeStarts[affectedEnd]:]
		replacements[affectedStart] = firstPrefix + newText
		for i := affectedStart + 1; i < affectedEnd; i++ {
			replacements[i] = ""
		}
		replacements[affectedEnd] = lastSuffix
	}

	updated := []byte(blockXML)
	for i := len(ranges) - 1; i >= 0; i-- {
		replacement, ok := replacements[i]
		if !ok {
			continue
		}
		escaped := escapeXMLText(replacement)
		textRange := ranges[i]
		updated = append(updated[:textRange.start], append([]byte(escaped), updated[textRange.end:]...)...)
	}

	return string(updated), nil
}

func textRangesInBlockXML(blockXML string) ([]textRange, string, error) {
	data := []byte(blockXML)
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var ranges []textRange
	var fullText strings.Builder
	textStart := -1

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("parse block xml: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				textStart = int(decoder.InputOffset())
			}
		case xml.EndElement:
			if t.Name.Local == "t" && textStart >= 0 {
				textEnd := startTagOffset(data, int(decoder.InputOffset()))
				raw := string(data[textStart:textEnd])
				decoded := decodeXMLText(raw)
				ranges = append(ranges, textRange{
					start: textStart,
					end:   textEnd,
					text:  decoded,
				})
				fullText.WriteString(decoded)
				textStart = -1
			}
		}
	}

	return ranges, fullText.String(), nil
}

func decodeXMLText(raw string) string {
	decoder := xml.NewDecoder(strings.NewReader("<x>" + raw + "</x>"))
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if charData, ok := token.(xml.CharData); ok {
			builder.Write([]byte(charData))
		}
	}
	return builder.String()
}

func escapeXMLText(text string) string {
	var builder strings.Builder
	if err := xml.EscapeText(&builder, []byte(text)); err != nil {
		return text
	}
	return builder.String()
}
