package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kiry163/docxtidy"
)

func TestCLIExtractWritesSnapshot(t *testing.T) {
	workDir := t.TempDir()
	inputPath := writeMainTestDocx(t, workDir)
	outPath := filepath.Join(workDir, "snapshot.json")

	if err := run([]string{"extract", inputPath, "--out", outPath}); err != nil {
		t.Fatalf("run extract returned error: %v", err)
	}

	snapshot := readMainTestSnapshot(t, outPath)
	if len(snapshot.Package.Parts) == 0 {
		t.Fatal("snapshot has no package parts")
	}
	if len(snapshot.Document.Blocks) == 0 {
		t.Fatal("snapshot has no document blocks")
	}
}

func TestCLIExtractWritesReadableXMLInJSON(t *testing.T) {
	workDir := t.TempDir()
	inputPath := writeMainTestDocx(t, workDir)
	outPath := filepath.Join(workDir, "snapshot.json")

	if err := run([]string{"extract", inputPath, "--out", outPath}); err != nil {
		t.Fatalf("run extract returned error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read snapshot json: %v", err)
	}
	jsonText := string(data)
	if !strings.Contains(jsonText, `"<w:p`) {
		t.Fatalf("snapshot json does not contain readable XML start tag")
	}
	if strings.Contains(jsonText, `\u003c`) || strings.Contains(jsonText, `\u003e`) {
		t.Fatalf("snapshot json contains HTML-escaped XML")
	}
	if strings.Contains(jsonText, `"document_id"`) {
		t.Fatalf("snapshot json contains document_id")
	}
}

func TestCLIOutlineWritesDocumentOutline(t *testing.T) {
	workDir := t.TempDir()
	inputPath := writeMainTestDocx(t, workDir)
	snapshotPath := filepath.Join(workDir, "snapshot.json")
	outlinePath := filepath.Join(workDir, "outline.json")
	if err := run([]string{"extract", inputPath, "--out", snapshotPath}); err != nil {
		t.Fatalf("run extract returned error: %v", err)
	}

	if err := run([]string{"outline", snapshotPath, "--out", outlinePath}); err != nil {
		t.Fatalf("run outline returned error: %v", err)
	}

	outline := readMainTestOutline(t, outlinePath)
	if len(outline.Blocks) == 0 {
		t.Fatal("outline has no blocks")
	}
	if outline.Blocks[0].ID == "" {
		t.Fatal("first outline block id is empty")
	}
	data, err := os.ReadFile(outlinePath)
	if err != nil {
		t.Fatalf("read outline json: %v", err)
	}
	if strings.Contains(string(data), `"document_id"`) {
		t.Fatalf("outline json contains document_id")
	}
}

func TestCLIApplyWritesUpdatedSnapshot(t *testing.T) {
	workDir := t.TempDir()
	inputPath := writeMainTestDocx(t, workDir)
	snapshotPath := filepath.Join(workDir, "snapshot.json")
	layoutPath := filepath.Join(workDir, "layout.json")
	updatedPath := filepath.Join(workDir, "updated-snapshot.json")
	writeMainTestJSON(t, layoutPath, mainTestLayout())
	if err := run([]string{"extract", inputPath, "--out", snapshotPath}); err != nil {
		t.Fatalf("run extract returned error: %v", err)
	}

	if err := run([]string{
		"apply",
		snapshotPath,
		"--layout",
		layoutPath,
		"--out",
		updatedPath,
	}); err != nil {
		t.Fatalf("run apply returned error: %v", err)
	}

	snapshot := readMainTestSnapshot(t, updatedPath)
	if snapshot.Document.Blocks[0].ID != "block-0004" {
		t.Fatalf("first block = %q, want block-0004", snapshot.Document.Blocks[0].ID)
	}
	if snapshot.Document.Blocks[4].Text[:len("【摘要：】")] != "【摘要：】" {
		t.Fatalf("abstract block text = %q, want standardized prefix", snapshot.Document.Blocks[4].Text)
	}
}

func TestCLIWriteCreatesDocxFromSnapshot(t *testing.T) {
	workDir := t.TempDir()
	inputPath := writeMainTestDocx(t, workDir)
	snapshotPath := filepath.Join(workDir, "snapshot.json")
	outputPath := filepath.Join(workDir, "output.docx")
	if err := run([]string{"extract", inputPath, "--out", snapshotPath}); err != nil {
		t.Fatalf("run extract returned error: %v", err)
	}

	if err := run([]string{"write", snapshotPath, "--out", outputPath}); err != nil {
		t.Fatalf("run write returned error: %v", err)
	}

	reader, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("written docx is not readable: %v", err)
	}
	defer reader.Close()

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

func readMainTestSnapshot(t *testing.T, snapshotPath string) docxtidy.Snapshot {
	t.Helper()

	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	var snapshot docxtidy.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	return snapshot
}

func readMainTestOutline(t *testing.T, outlinePath string) docxtidy.Outline {
	t.Helper()

	data, err := os.ReadFile(outlinePath)
	if err != nil {
		t.Fatalf("read outline: %v", err)
	}

	var outline docxtidy.Outline
	if err := json.Unmarshal(data, &outline); err != nil {
		t.Fatalf("decode outline: %v", err)
	}
	return outline
}

func writeMainTestDocx(t *testing.T, dir string) string {
	t.Helper()

	path := filepath.Join(dir, "input.docx")
	if err := os.WriteFile(path, mainTestDocx(t), 0o644); err != nil {
		t.Fatalf("write test docx: %v", err)
	}
	return path
}

func mainTestDocx(t *testing.T) []byte {
	t.Helper()

	var body strings.Builder
	for i := 1; i <= 53; i++ {
		switch i {
		case 10:
			body.WriteString(mainTestParagraphXML("摘要：本文概述 DocxTidy 的测试夹具。"))
		case 11:
			body.WriteString(mainTestParagraphXML("关键词：DOCX；整理；测试"))
		case 53:
			body.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>`)
		default:
			body.WriteString(mainTestParagraphXML("block text"))
		}
	}

	documentXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` + body.String() + `</w:body></w:document>`

	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	mainTestZipEntry(t, writer, "[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`)
	mainTestZipEntry(t, writer, "word/document.xml", documentXML)
	if err := writer.Close(); err != nil {
		t.Fatalf("close test docx: %v", err)
	}
	return output.Bytes()
}

func mainTestParagraphXML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return `<w:p><w:r><w:t>` + text + `</w:t></w:r></w:p>`
}

func mainTestZipEntry(t *testing.T, writer *zip.Writer, name string, data string) {
	t.Helper()
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatalf("create test docx entry %s: %v", name, err)
	}
	if _, err := entry.Write([]byte(data)); err != nil {
		t.Fatalf("write test docx entry %s: %v", name, err)
	}
}

func writeMainTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("encode json fixture: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write json fixture: %v", err)
	}
}

func mainTestLayout() docxtidy.Layout {
	return docxtidy.Layout{
		Groups: []docxtidy.Group{
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
		Edits: []docxtidy.Edit{
			{BlockID: "block-0010", Replace: &docxtidy.TextReplacement{Old: "摘要：", New: "【摘要：】"}},
			{BlockID: "block-0011", Replace: &docxtidy.TextReplacement{Old: "关键词：", New: "【关键词：】"}},
		},
	}
}
