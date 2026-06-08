package docxtidy

import (
	"archive/zip"
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestExtractReturnsStateFromReader(t *testing.T) {
	state, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{DocumentID: "sample"})
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
	state, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{DocumentID: "sample"})
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

func TestApplyReturnsNewStateAndPreservesInput(t *testing.T) {
	state, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{DocumentID: "sample"})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	originalFirstID := state.Document.Blocks[0].ID
	originalAbstractText := state.Document.Blocks[9].Text

	updated, err := Apply(context.Background(), state, standardTestStructure(), standardTestTransform())
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
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
	state, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{DocumentID: "sample"})
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

func TestBodyBlocksDisplayTextUsesImagePlaceholder(t *testing.T) {
	documentXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
    xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
    xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
    xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:inline>
            <wp:docPr id="1" name="图片 1"/>
            <a:graphic>
              <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
                <pic:pic>
                  <pic:blipFill>
                    <a:blip r:embed="rId5"/>
                  </pic:blipFill>
                </pic:pic>
              </a:graphicData>
            </a:graphic>
          </wp:inline>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`)

	state, err := Extract(context.Background(), bytes.NewReader(docxWithDocumentXML(t, string(documentXML))), ExtractOptions{})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(state.Document.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(state.Document.Blocks))
	}
	block := state.Document.Blocks[0]
	if block.Type != BlockTypeParagraph {
		t.Fatalf("block type = %q, want paragraph", block.Type)
	}
	if block.Text != "" {
		t.Fatalf("block text = %q, want empty source text", block.Text)
	}
	if block.DisplayText != "![图片 1](media:rId5)" {
		t.Fatalf("block display text = %q, want image placeholder", block.DisplayText)
	}
}

func TestBodyBlocksDisplayTextCombinesTextAndImagePlaceholder(t *testing.T) {
	documentXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
    xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
    xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:r><w:t>见下图：</w:t></w:r>
      <w:r>
        <w:drawing>
          <wp:inline>
            <wp:docPr id="1" name="图片 1"/>
            <a:graphic><a:graphicData><a:blip r:embed="rId5"/></a:graphicData></a:graphic>
          </wp:inline>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`)

	state, err := Extract(context.Background(), bytes.NewReader(docxWithDocumentXML(t, string(documentXML))), ExtractOptions{})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	block := state.Document.Blocks[0]
	if block.Text != "见下图：" {
		t.Fatalf("block text = %q, want source text only", block.Text)
	}
	if block.DisplayText != "见下图：![图片 1](media:rId5)" {
		t.Fatalf("block display text = %q, want text plus image placeholder", block.DisplayText)
	}
}

func TestViewOfReturnsLightweightDocumentView(t *testing.T) {
	state, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{DocumentID: "sample"})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	view := ViewOf(state, ViewOptions{})

	if view.DocumentID != "sample" {
		t.Fatalf("view document id = %q, want sample", view.DocumentID)
	}
	if len(view.Blocks) != len(state.Document.Blocks) {
		t.Fatalf("view blocks = %d, want %d", len(view.Blocks), len(state.Document.Blocks))
	}
	first := view.Blocks[0]
	if first.ID != state.Document.Blocks[0].ID {
		t.Fatalf("first view block id = %q, want %q", first.ID, state.Document.Blocks[0].ID)
	}
	if first.Index != 0 {
		t.Fatalf("first view block index = %d, want 0", first.Index)
	}
	if first.Text != state.Document.Blocks[0].DisplayText {
		t.Fatalf("first view block text = %q, want %q", first.Text, state.Document.Blocks[0].DisplayText)
	}
	numbered := viewBlockByIDForTest(t, view, "block-0016")
	if !strings.HasPrefix(numbered.Text, "1.2 ") {
		t.Fatalf("numbered view text = %q, want 1.2 prefix", numbered.Text)
	}
}

func TestViewOfCanUseSourceText(t *testing.T) {
	state, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{DocumentID: "sample"})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	view := ViewOf(state, ViewOptions{TextMode: ViewTextModeSource})

	numbered := viewBlockByIDForTest(t, view, "block-0016")
	if strings.HasPrefix(numbered.Text, "1.2 ") {
		t.Fatalf("numbered source text = %q, want no computed numbering prefix", numbered.Text)
	}
	source := blockByIDForTest(t, state, "block-0016")
	if numbered.Text != source.Text {
		t.Fatalf("numbered source text = %q, want %q", numbered.Text, source.Text)
	}
}

func TestViewOfDisplaysTablesAsMarkdown(t *testing.T) {
	state, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{DocumentID: "sample"})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	tableIndex := firstBlockIndexByTypeForTest(t, state, BlockTypeTable)
	displayView := ViewOf(state, ViewOptions{})
	sourceView := ViewOf(state, ViewOptions{TextMode: ViewTextModeSource})

	displayText := displayView.Blocks[tableIndex].Text
	if !strings.Contains(displayText, "| 问题类型 | 具体表现 | 影响程度 |") {
		t.Fatalf("display table text = %q, want markdown header row", displayText)
	}
	if !strings.Contains(displayText, "| --- | --- | --- |") {
		t.Fatalf("display table text = %q, want markdown separator row", displayText)
	}
	if !strings.Contains(displayText, "| 产教融合问题 | 企业参与度不高 | 严重 |") {
		t.Fatalf("display table text = %q, want markdown body row", displayText)
	}

	sourceText := sourceView.Blocks[tableIndex].Text
	if strings.Contains(sourceText, "| --- |") {
		t.Fatalf("source table text = %q, want source text without markdown separator", sourceText)
	}
	if sourceText != state.Document.Blocks[tableIndex].Text {
		t.Fatalf("source table text = %q, want %q", sourceText, state.Document.Blocks[tableIndex].Text)
	}
}

func TestViewOfDisplaysTableImagesAsMarkdownPlaceholders(t *testing.T) {
	state := State{
		Document: Document{
			ID: "sample",
			Blocks: []Block{
				{
					ID:   "block-0001",
					Type: BlockTypeTable,
					Text: "",
					XML: `<w:tbl xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
    xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
    xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:tr>
    <w:tc><w:p><w:r><w:t>图片</w:t></w:r></w:p></w:tc>
  </w:tr>
  <w:tr>
    <w:tc><w:p><w:r><w:drawing><wp:inline><wp:docPr id="1" name="图片 1"/><a:graphic><a:graphicData><a:blip r:embed="rId5"/></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p></w:tc>
  </w:tr>
</w:tbl>`,
				},
			},
		},
	}

	view := ViewOf(state, ViewOptions{})
	tableText := view.Blocks[0].Text
	if !strings.Contains(tableText, "| 图片 |") {
		t.Fatalf("table text = %q, want markdown header", tableText)
	}
	if !strings.Contains(tableText, "| ![图片 1](media:rId5) |") {
		t.Fatalf("table text = %q, want image placeholder cell", tableText)
	}
}

func sampleDocx(t *testing.T) []byte {
	t.Helper()

	var body strings.Builder
	for i := 1; i <= 53; i++ {
		switch i {
		case 10:
			body.WriteString(paragraphXML("摘要：本文概述 DocxTidy 的测试夹具。"))
		case 11:
			body.WriteString(paragraphXML("关键词：DOCX；整理；测试"))
		case 12:
			body.WriteString(tableXML())
		case 15, 16:
			body.WriteString(numberedParagraphXML("编号段落", "1", 1))
		case 53:
			body.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>`)
		default:
			body.WriteString(paragraphXML("block text"))
		}
	}

	documentXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` + body.String() + `</w:body></w:document>`
	numberingXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:abstractNum w:abstractNumId="1">` +
		`<w:lvl w:ilvl="1"><w:lvlText w:val="%1.%2"/></w:lvl>` +
		`</w:abstractNum>` +
		`<w:num w:numId="1"><w:abstractNumId w:val="1"/></w:num>` +
		`</w:numbering>`

	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	writeZipEntry(t, writer, "[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`)
	writeZipEntry(t, writer, "word/document.xml", documentXML)
	writeZipEntry(t, writer, "word/numbering.xml", numberingXML)
	if err := writer.Close(); err != nil {
		t.Fatalf("close sample docx: %v", err)
	}
	return output.Bytes()
}

func docxWithDocumentXML(t *testing.T, documentXML string) []byte {
	t.Helper()

	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	writeZipEntry(t, writer, "[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`)
	writeZipEntry(t, writer, "word/document.xml", documentXML)
	if err := writer.Close(); err != nil {
		t.Fatalf("close sample docx: %v", err)
	}
	return output.Bytes()
}

func paragraphXML(text string) string {
	return `<w:p><w:r><w:t>` + escapeFixtureText(text) + `</w:t></w:r></w:p>`
}

func numberedParagraphXML(text string, numID string, level int) string {
	return `<w:p><w:pPr><w:numPr><w:ilvl w:val="` + strconv.Itoa(level) + `"/><w:numId w:val="` + numID + `"/></w:numPr></w:pPr><w:r><w:t>` + escapeFixtureText(text) + `</w:t></w:r></w:p>`
}

func tableXML() string {
	return `<w:tbl>` +
		`<w:tr><w:tc><w:p><w:r><w:t>问题类型</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>具体表现</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>影响程度</w:t></w:r></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:r><w:t>产教融合问题</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>企业参与度不高</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>严重</w:t></w:r></w:p></w:tc></w:tr>` +
		`</w:tbl>`
}

func escapeFixtureText(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	return strings.ReplaceAll(text, ">", "&gt;")
}

func writeZipEntry(t *testing.T, writer *zip.Writer, name string, data string) {
	t.Helper()
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatalf("create sample docx entry %s: %v", name, err)
	}
	if _, err := entry.Write([]byte(data)); err != nil {
		t.Fatalf("write sample docx entry %s: %v", name, err)
	}
}

func stateHasPackageFile(state State, name string) bool {
	for _, file := range state.Files {
		if file.Name == name {
			return true
		}
	}
	return false
}

func blockByIDForTest(t *testing.T, state State, blockID string) Block {
	t.Helper()
	for _, block := range state.Document.Blocks {
		if block.ID == blockID {
			return block
		}
	}
	t.Fatalf("block %s not found", blockID)
	return Block{}
}

func firstBlockIndexByTypeForTest(t *testing.T, state State, blockType BlockType) int {
	t.Helper()
	for index, block := range state.Document.Blocks {
		if block.Type == blockType {
			return index
		}
	}
	t.Fatalf("block type %s not found", blockType)
	return -1
}

func viewBlockByIDForTest(t *testing.T, view View, blockID string) ViewBlock {
	t.Helper()
	for _, block := range view.Blocks {
		if block.ID == blockID {
			return block
		}
	}
	t.Fatalf("view block %s not found", blockID)
	return ViewBlock{}
}

func standardTestTransform() Transform {
	return Transform{
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
		TextEdits: []TextEdit{
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
