package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kiry163/docxtidy"
)

func TestCLIExtractWritesDocumentState(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "state.json")

	if err := run([]string{"extract", "../../test.docx", "--out", outPath}); err != nil {
		t.Fatalf("run extract returned error: %v", err)
	}

	state := readMainTestState(t, outPath)
	if len(state.Files) == 0 {
		t.Fatal("state has no package files")
	}
	if len(state.Document.Blocks) == 0 {
		t.Fatal("state has no document blocks")
	}
}

func TestCLIApplyLayoutWritesUpdatedState(t *testing.T) {
	workDir := t.TempDir()
	statePath := filepath.Join(workDir, "state.json")
	updatedPath := filepath.Join(workDir, "updated-state.json")
	if err := run([]string{"extract", "../../test.docx", "--out", statePath}); err != nil {
		t.Fatalf("run extract returned error: %v", err)
	}

	if err := run([]string{
		"apply-layout",
		statePath,
		"--structure",
		"../../examples/test.structure.json",
		"--layout",
		"../../examples/layout.standard.json",
		"--out",
		updatedPath,
	}); err != nil {
		t.Fatalf("run apply-layout returned error: %v", err)
	}

	state := readMainTestState(t, updatedPath)
	if state.Document.Blocks[0].ID != "block-0004" {
		t.Fatalf("first block = %q, want block-0004", state.Document.Blocks[0].ID)
	}
	if state.Document.Blocks[4].Text[:len("【摘要：】")] != "【摘要：】" {
		t.Fatalf("abstract block text = %q, want standardized prefix", state.Document.Blocks[4].Text)
	}
}

func TestCLIWriteCreatesDocxFromState(t *testing.T) {
	workDir := t.TempDir()
	statePath := filepath.Join(workDir, "state.json")
	outputPath := filepath.Join(workDir, "output.docx")
	if err := run([]string{"extract", "../../test.docx", "--out", statePath}); err != nil {
		t.Fatalf("run extract returned error: %v", err)
	}

	if err := run([]string{"write", statePath, "--out", outputPath}); err != nil {
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

func readMainTestState(t *testing.T, statePath string) docxtidy.DocumentState {
	t.Helper()

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	var state docxtidy.DocumentState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	return state
}
