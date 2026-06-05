package docxtidy

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestExtractReturnsDocumentStateFromReader(t *testing.T) {
	input, err := os.Open("test.docx")
	if err != nil {
		t.Fatalf("open test docx: %v", err)
	}
	defer input.Close()

	state, err := Extract(context.Background(), input, ExtractOptions{DocumentID: "sample"})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	if state.Document.ID != "sample" {
		t.Fatalf("document id = %q, want sample", state.Document.ID)
	}
	if len(state.Files) == 0 {
		t.Fatal("state has no package files")
	}
	if len(state.Document.Blocks) == 0 {
		t.Fatal("state has no document blocks")
	}
	if state.Document.Blocks[0].ID == "" {
		t.Fatal("first block ID is empty")
	}
	if state.Document.Blocks[0].XML == "" {
		t.Fatal("first block XML is empty")
	}
	if !stateHasPackageFile(state, "word/document.xml") {
		t.Fatal("state is missing word/document.xml")
	}
}

func TestWriteRoundTripsExtractedState(t *testing.T) {
	input, err := os.Open("test.docx")
	if err != nil {
		t.Fatalf("open test docx: %v", err)
	}
	defer input.Close()

	state, err := Extract(context.Background(), input, ExtractOptions{DocumentID: "sample"})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	var output bytes.Buffer
	if err := Write(context.Background(), state, &output); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatalf("written docx is not a readable zip: %v", err)
	}

	foundDocumentXML := false
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			foundDocumentXML = true
			break
		}
	}
	if !foundDocumentXML {
		t.Fatal("written docx is missing word/document.xml")
	}
}

func TestApplyLayoutReturnsNewStateAndPreservesInput(t *testing.T) {
	input, err := os.Open("test.docx")
	if err != nil {
		t.Fatalf("open test docx: %v", err)
	}
	defer input.Close()

	state, err := Extract(context.Background(), input, ExtractOptions{DocumentID: "sample"})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	originalFirstID := state.Document.Blocks[0].ID
	originalAbstractText := state.Document.Blocks[9].Text

	updated, err := ApplyLayout(context.Background(), state, standardTestStructure(), standardTestLayout())
	if err != nil {
		t.Fatalf("ApplyLayout returned error: %v", err)
	}

	expectedLeadingIDs := []string{
		"block-0004",
		"block-0006",
		"block-0007",
		"block-0008",
		"block-0010",
		"block-0011",
	}
	for i, expectedID := range expectedLeadingIDs {
		if updated.Document.Blocks[i].ID != expectedID {
			t.Fatalf("updated block at index %d = %q, want %q", i, updated.Document.Blocks[i].ID, expectedID)
		}
	}
	if !strings.HasPrefix(updated.Document.Blocks[4].Text, "【摘要：】") {
		t.Fatalf("abstract text = %q, want standardized prefix", updated.Document.Blocks[4].Text)
	}
	if !strings.HasPrefix(updated.Document.Blocks[5].Text, "【关键词：】") {
		t.Fatalf("keywords text = %q, want standardized prefix", updated.Document.Blocks[5].Text)
	}
	if state.Document.Blocks[0].ID != originalFirstID {
		t.Fatalf("original first block changed to %q, want %q", state.Document.Blocks[0].ID, originalFirstID)
	}
	if state.Document.Blocks[9].Text != originalAbstractText {
		t.Fatalf("original abstract text changed to %q, want %q", state.Document.Blocks[9].Text, originalAbstractText)
	}
}

func TestExtractCapturesAutomaticNumberingMetadata(t *testing.T) {
	input, err := os.Open("test.docx")
	if err != nil {
		t.Fatalf("open test docx: %v", err)
	}
	defer input.Close()

	state, err := Extract(context.Background(), input, ExtractOptions{DocumentID: "sample"})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	block := blockByIDForTest(t, state, "block-0016")
	if block.Numbering == nil {
		t.Fatal("block-0016 numbering is nil")
	}
	if block.Numbering.Kind != NumberingKindAuto {
		t.Fatalf("numbering kind = %q, want %q", block.Numbering.Kind, NumberingKindAuto)
	}
	if block.Numbering.NumID != "1" {
		t.Fatalf("numbering num id = %q, want 1", block.Numbering.NumID)
	}
	if block.Numbering.Level != 1 {
		t.Fatalf("numbering level = %d, want 1", block.Numbering.Level)
	}
	if block.Numbering.LevelText != "%1.%2" {
		t.Fatalf("numbering level text = %q, want %%1.%%2", block.Numbering.LevelText)
	}
	if block.Numbering.ComputedLabel != "1.2" {
		t.Fatalf("computed label = %q, want 1.2", block.Numbering.ComputedLabel)
	}
	if !strings.HasPrefix(block.DisplayText, "1.2 ") {
		t.Fatalf("display text = %q, want 1.2 prefix", block.DisplayText)
	}
}

func stateHasPackageFile(state DocumentState, name string) bool {
	for _, file := range state.Files {
		if file.Name == name {
			return true
		}
	}
	return false
}

func blockByIDForTest(t *testing.T, state DocumentState, blockID string) Block {
	t.Helper()
	for _, block := range state.Document.Blocks {
		if block.ID == blockID {
			return block
		}
	}
	t.Fatalf("block %s not found", blockID)
	return Block{}
}

func standardTestLayout() Layout {
	return Layout{
		Order: []string{
			"title",
			"author",
			"affiliation",
			"abstract",
			"keywords",
			"body",
			"references",
			"front_matter",
			"tail",
		},
		TextReplacements: []TextReplacement{
			{Role: "abstract", Old: "摘要：", New: "【摘要：】"},
			{Role: "keywords", Old: "关键词：", New: "【关键词：】"},
		},
	}
}

func standardTestStructure() Structure {
	return Structure{
		Sections: []Section{
			{Role: "front_matter", BlockIDs: []string{"block-0001", "block-0002", "block-0003", "block-0005", "block-0009"}},
			{Role: "title", BlockIDs: []string{"block-0004"}},
			{Role: "author", BlockIDs: []string{"block-0006", "block-0007"}},
			{Role: "affiliation", BlockIDs: []string{"block-0008"}},
			{Role: "abstract", BlockIDs: []string{"block-0010"}},
			{Role: "keywords", BlockIDs: []string{"block-0011"}},
			{Role: "body", BlockIDs: []string{
				"block-0012",
				"block-0013",
				"block-0014",
				"block-0015",
				"block-0016",
				"block-0017",
				"block-0018",
				"block-0019",
				"block-0020",
				"block-0021",
				"block-0022",
				"block-0023",
				"block-0024",
				"block-0025",
				"block-0026",
				"block-0027",
				"block-0028",
				"block-0029",
				"block-0030",
				"block-0031",
				"block-0032",
				"block-0033",
				"block-0034",
				"block-0035",
				"block-0036",
				"block-0037",
				"block-0038",
				"block-0039",
				"block-0040",
				"block-0041",
				"block-0042",
				"block-0043",
				"block-0044",
				"block-0045",
				"block-0046",
			}},
			{Role: "references", BlockIDs: []string{"block-0047", "block-0048", "block-0049", "block-0050", "block-0051", "block-0052"}},
			{Role: "tail", BlockIDs: []string{"block-0053"}},
		},
	}
}
