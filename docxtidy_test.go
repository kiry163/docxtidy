package docxtidy

import (
	"archive/zip"
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestExtractReturnsSnapshotFromReader(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	if len(snapshot.Package.Parts) == 0 {
		t.Fatal("snapshot has no package parts")
	}
	if len(snapshot.Document.Blocks) == 0 {
		t.Fatal("snapshot has no document blocks")
	}
	if snapshot.Document.Blocks[0].ID == "" {
		t.Fatal("first block ID is empty")
	}
	if snapshot.Document.Blocks[0].XML == "" {
		t.Fatal("first block XML is empty")
	}
	if !snapshotHasPackagePart(snapshot, "word/document.xml") {
		t.Fatal("snapshot is missing word/document.xml")
	}
}

func TestWriteRoundTripsExtractedSnapshot(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	var output bytes.Buffer
	if err := Write(context.Background(), snapshot, &output); err != nil {
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

func TestApplyReturnsNewSnapshotFromLayoutAndPreservesInput(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	originalFirstID := snapshot.Document.Blocks[0].ID
	originalAbstractText := snapshot.Document.Blocks[9].Text

	updated, err := Apply(context.Background(), snapshot, standardTestLayout())
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
	if snapshot.Document.Blocks[0].ID != originalFirstID {
		t.Fatalf("original first block changed to %q, want %q", snapshot.Document.Blocks[0].ID, originalFirstID)
	}
	if snapshot.Document.Blocks[9].Text != originalAbstractText {
		t.Fatalf("original abstract text changed to %q, want %q", snapshot.Document.Blocks[9].Text, originalAbstractText)
	}
}

func TestApplyRejectsMissingBlockInLayout(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Groups[0].BlockIDs = layout.Groups[0].BlockIDs[1:]

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "missing block") {
		t.Fatalf("Apply error = %v, want missing block error", err)
	}
}

func TestApplyRejectsDuplicateBlockInLayout(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Groups[0].BlockIDs[0] = layout.Groups[0].BlockIDs[1]

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "appears more than once") {
		t.Fatalf("Apply error = %v, want duplicate block error", err)
	}
}

func TestApplyRejectsUnknownBlockInLayout(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Groups[0].BlockIDs[0] = "missing-block"

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "unknown block") {
		t.Fatalf("Apply error = %v, want unknown block error", err)
	}
}

func TestApplyRejectsEmptyLayoutGroup(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Groups = append([]Group{{}}, layout.Groups...)

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "empty group") {
		t.Fatalf("Apply error = %v, want empty group error", err)
	}
}

func TestApplyRejectsProtectedBlockEdit(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Edits = []Edit{{BlockID: "block-0053", Replace: &TextReplacement{Old: "11906", New: "1"}}}

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "protected block") {
		t.Fatalf("Apply error = %v, want protected block error", err)
	}
}

func TestApplyRejectsMissingReplacement(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Edits = []Edit{{BlockID: "block-0010"}}

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "missing replacement") {
		t.Fatalf("Apply error = %v, want missing replacement error", err)
	}
}

func TestApplyRejectsMissingOldText(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Edits = []Edit{{BlockID: "block-0010", Replace: &TextReplacement{Old: "不存在", New: "新文本"}}}

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "cannot find") {
		t.Fatalf("Apply error = %v, want cannot find error", err)
	}
}

func TestApplyPreservesAutomaticNumberingByDefault(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)

	updated, err := Apply(context.Background(), snapshot, layout)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	block := blockByIDForTest(t, updated, "block-0016")
	if !strings.Contains(block.XML, "<w:numPr>") {
		t.Fatalf("block xml = %s, want automatic numbering preserved", block.XML)
	}
}

func TestApplyAutomaticNumberingTextMaterializesNumberedParagraphs(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.AutomaticNumbering = AutomaticNumberingText

	updated, err := Apply(context.Background(), snapshot, layout)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	block := blockByIDForTest(t, updated, "block-0016")
	if strings.Contains(block.XML, "<w:numPr>") {
		t.Fatalf("block xml = %s, want automatic numbering removed", block.XML)
	}
	if strings.Contains(block.XML, "<w:pStyle") {
		t.Fatalf("block xml = %s, want paragraph style removed", block.XML)
	}
	if strings.Contains(block.XML, "<w:ind") {
		t.Fatalf("block xml = %s, want paragraph indentation removed", block.XML)
	}
	if block.Text != "1.2编号段落" {
		t.Fatalf("block text = %q, want automatic numbering materialized", block.Text)
	}
	if block.DisplayText != block.Text {
		t.Fatalf("display text = %q, want %q", block.DisplayText, block.Text)
	}
}

func TestApplyAutomaticNumberingTextNormalizesNumberingSeparator(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	for i, block := range snapshot.Document.Blocks {
		if block.ID != "block-0016" {
			continue
		}
		block.Text = " 编号段落"
		block.DisplayText = "1.2  编号段落"
		block.XML = strings.Replace(block.XML, ">编号段落<", "> 编号段落<", 1)
		snapshot.Document.Blocks[i] = block
		break
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.AutomaticNumbering = AutomaticNumberingText

	updated, err := Apply(context.Background(), snapshot, layout)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	block := blockByIDForTest(t, updated, "block-0016")
	if block.Text != "1.2编号段落" {
		t.Fatalf("block text = %q, want no inserted separator", block.Text)
	}
}

func TestApplyAutomaticNumberingTextKeepsManualNumberingEditsAuthoritative(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.AutomaticNumbering = AutomaticNumberingText
	layout.Edits = []Edit{
		{
			BlockID: "block-0016",
			ManualNumbering: &ManualNumberingEdit{
				Text:  "手写 编号段落",
				Style: ManualNumberingStylePlain,
			},
		},
	}

	updated, err := Apply(context.Background(), snapshot, layout)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	block := blockByIDForTest(t, updated, "block-0016")
	if block.Text != "手写 编号段落" {
		t.Fatalf("block text = %q, want manual numbering edit text", block.Text)
	}
}

func TestApplyRejectsUnknownAutomaticNumberingPolicy(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.AutomaticNumbering = AutomaticNumberingPolicy("surprise")

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "unknown automatic numbering policy") {
		t.Fatalf("Apply error = %v, want unknown automatic numbering policy error", err)
	}
}

func TestApplyManualNumberingEditRebuildsNumberedParagraph(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Edits = []Edit{
		{
			BlockID: "block-0016",
			ManualNumbering: &ManualNumberingEdit{
				Text:  "1.2 编号段落",
				Style: ManualNumberingStyleHeading,
			},
		},
	}

	updated, err := Apply(context.Background(), snapshot, layout)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	block := blockByIDForTest(t, updated, "block-0016")
	if strings.Contains(block.XML, "<w:numPr>") {
		t.Fatalf("block xml = %s, want automatic numbering removed", block.XML)
	}
	if strings.Contains(block.XML, "<w:pStyle") {
		t.Fatalf("block xml = %s, want paragraph style removed", block.XML)
	}
	if strings.Contains(block.XML, "<w:ind") {
		t.Fatalf("block xml = %s, want paragraph indentation removed", block.XML)
	}
	if block.Text != "1.2 编号段落" {
		t.Fatalf("block text = %q, want manual numbering text", block.Text)
	}
	if !strings.Contains(block.XML, `<w:b/>`) {
		t.Fatalf("block xml = %s, want heading styling", block.XML)
	}
}

func TestApplyRejectsEditWithReplaceAndManualNumbering(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Edits = []Edit{
		{
			BlockID:         "block-0016",
			Replace:         &TextReplacement{Old: "编号段落", New: "1.2 编号段落"},
			ManualNumbering: &ManualNumberingEdit{Text: "1.2 编号段落"},
		},
	}

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "multiple edit actions") {
		t.Fatalf("Apply error = %v, want multiple edit actions error", err)
	}
}

func TestApplyRejectsManualNumberingEditWithoutText(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Edits = []Edit{{BlockID: "block-0016", ManualNumbering: &ManualNumberingEdit{}}}

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "empty manual numbering text") {
		t.Fatalf("Apply error = %v, want empty manual numbering text error", err)
	}
}

func TestExtractAddsAutomaticNumberingToDisplayText(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	block := blockByIDForTest(t, snapshot, "block-0016")
	if block.DisplayText != "1.2编号段落" {
		t.Fatalf("display text = %q, want numbering prefix without inserted space", block.DisplayText)
	}
}

func TestExtractUsesNumberingLevelStartValues(t *testing.T) {
	documentXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` + numberedParagraphXML("支撑保障", "2", 1) + `</w:body></w:document>`
	numberingXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:abstractNum w:abstractNumId="1">` +
		`<w:lvl w:ilvl="0"><w:start w:val="3"/><w:lvlText w:val="%1"/></w:lvl>` +
		`<w:lvl w:ilvl="1"><w:start w:val="4"/><w:lvlText w:val="%1.%2"/></w:lvl>` +
		`</w:abstractNum>` +
		`<w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>` +
		`</w:numbering>`

	snapshot, err := Extract(context.Background(), bytes.NewReader(docxWithDocumentAndNumberingXML(t, documentXML, numberingXML)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	block := blockByIDForTest(t, snapshot, "block-0001")
	if block.DisplayText != "3.4支撑保障" {
		t.Fatalf("display text = %q, want 3.4 prefix", block.DisplayText)
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

	snapshot, err := Extract(context.Background(), bytes.NewReader(docxWithDocumentXML(t, string(documentXML))))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(snapshot.Document.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(snapshot.Document.Blocks))
	}
	block := snapshot.Document.Blocks[0]
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

	snapshot, err := Extract(context.Background(), bytes.NewReader(docxWithDocumentXML(t, string(documentXML))))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	block := snapshot.Document.Blocks[0]
	if block.Text != "见下图：" {
		t.Fatalf("block text = %q, want source text only", block.Text)
	}
	if block.DisplayText != "见下图：![图片 1](media:rId5)" {
		t.Fatalf("block display text = %q, want text plus image placeholder", block.DisplayText)
	}
}

func TestOutlineOfReturnsLightweightDocumentOutline(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	outline := OutlineOf(snapshot)

	if len(outline.Blocks) != len(snapshot.Document.Blocks) {
		t.Fatalf("outline blocks = %d, want %d", len(outline.Blocks), len(snapshot.Document.Blocks))
	}
	first := outline.Blocks[0]
	if first.ID != snapshot.Document.Blocks[0].ID {
		t.Fatalf("first outline block id = %q, want %q", first.ID, snapshot.Document.Blocks[0].ID)
	}
	if first.Index != 0 {
		t.Fatalf("first outline block index = %d, want 0", first.Index)
	}
	if first.Text != snapshot.Document.Blocks[0].DisplayText {
		t.Fatalf("first outline block text = %q, want %q", first.Text, snapshot.Document.Blocks[0].DisplayText)
	}
	numbered := outlineBlockByIDForTest(t, outline, "block-0016")
	if numbered.Text != "1.2编号段落" {
		t.Fatalf("numbered outline text = %q, want numbering prefix without inserted space", numbered.Text)
	}
}

func TestOutlineOfDisplaysTablesAsMarkdown(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	tableIndex := firstBlockIndexByTypeForTest(t, snapshot, BlockTypeTable)
	outline := OutlineOf(snapshot)

	displayText := outline.Blocks[tableIndex].Text
	if !strings.Contains(displayText, "| 问题类型 | 具体表现 | 影响程度 |") {
		t.Fatalf("display table text = %q, want markdown header row", displayText)
	}
	if !strings.Contains(displayText, "| --- | --- | --- |") {
		t.Fatalf("display table text = %q, want markdown separator row", displayText)
	}
	if !strings.Contains(displayText, "| 产教融合问题 | 企业参与度不高 | 严重 |") {
		t.Fatalf("display table text = %q, want markdown body row", displayText)
	}
}

func TestOutlineOfDisplaysTableImagesAsMarkdownPlaceholders(t *testing.T) {
	snapshot := Snapshot{
		Document: DocumentSnapshot{
			Blocks: []SnapshotBlock{
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

	outline := OutlineOf(snapshot)
	tableText := outline.Blocks[0].Text
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

	return docxWithDocumentAndNumberingXML(t, documentXML, "")
}

func docxWithDocumentAndNumberingXML(t *testing.T, documentXML string, numberingXML string) []byte {
	t.Helper()

	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	writeZipEntry(t, writer, "[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`)
	writeZipEntry(t, writer, "word/document.xml", documentXML)
	if numberingXML != "" {
		writeZipEntry(t, writer, "word/numbering.xml", numberingXML)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close sample docx: %v", err)
	}
	return output.Bytes()
}

func paragraphXML(text string) string {
	return `<w:p><w:r><w:t>` + escapeFixtureText(text) + `</w:t></w:r></w:p>`
}

func numberedParagraphXML(text string, numID string, level int) string {
	return `<w:p><w:pPr><w:pStyle w:val="5"/><w:numPr><w:ilvl w:val="` + strconv.Itoa(level) + `"/><w:numId w:val="` + numID + `"/></w:numPr><w:ind w:left="370" w:hanging="370"/></w:pPr><w:r><w:t>` + escapeFixtureText(text) + `</w:t></w:r></w:p>`
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

func snapshotHasPackagePart(snapshot Snapshot, name string) bool {
	for _, part := range snapshot.Package.Parts {
		if part.Name == name {
			return true
		}
	}
	return false
}

func blockByIDForTest(t *testing.T, snapshot Snapshot, blockID string) SnapshotBlock {
	t.Helper()
	for _, block := range snapshot.Document.Blocks {
		if block.ID == blockID {
			return block
		}
	}
	t.Fatalf("block %s not found", blockID)
	return SnapshotBlock{}
}

func firstBlockIndexByTypeForTest(t *testing.T, snapshot Snapshot, blockType BlockType) int {
	t.Helper()
	for index, block := range snapshot.Document.Blocks {
		if block.Type == blockType {
			return index
		}
	}
	t.Fatalf("block type %s not found", blockType)
	return -1
}

func outlineBlockByIDForTest(t *testing.T, outline Outline, blockID string) OutlineBlock {
	t.Helper()
	for _, block := range outline.Blocks {
		if block.ID == blockID {
			return block
		}
	}
	t.Fatalf("outline block %s not found", blockID)
	return OutlineBlock{}
}

func standardTestLayout() Layout {
	return Layout{
		Groups: []Group{
			{BlockIDs: []string{"block-0004"}},
			{BlockIDs: []string{"block-0006", "block-0007"}},
			{BlockIDs: []string{"block-0008"}},
			{BlockIDs: []string{"block-0010"}},
			{BlockIDs: []string{"block-0011"}},
			{BlockIDs: []string{
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
			{BlockIDs: []string{"block-0047", "block-0048", "block-0049", "block-0050", "block-0051", "block-0052"}},
			{BlockIDs: []string{"block-0001", "block-0002", "block-0003", "block-0005", "block-0009"}},
			{BlockIDs: []string{"block-0053"}},
		},
		Edits: []Edit{
			{BlockID: "block-0010", Replace: &TextReplacement{Old: "摘要：", New: "【摘要：】"}},
			{BlockID: "block-0011", Replace: &TextReplacement{Old: "关键词：", New: "【关键词：】"}},
		},
	}
}

func completeLayoutFromSnapshot(snapshot Snapshot) Layout {
	blockIDs := make([]string, 0, len(snapshot.Document.Blocks))
	for _, block := range snapshot.Document.Blocks {
		blockIDs = append(blockIDs, block.ID)
	}
	return Layout{Groups: []Group{{BlockIDs: blockIDs}}}
}
