# Snapshot Outline Layout API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the broad state/view/structure/transform API with a focused Snapshot -> Outline -> Layout -> Snapshot workflow for DOCX restructuring.

**Architecture:** Keep `internal/ooxml` as the engine and update the root package to expose workflow-oriented types. `Snapshot` remains the complete JSON-serializable write-back carrier, `Outline` is the compact model-facing projection, and `Layout` is the complete target arrangement plus explicit text replacements. The CLI mirrors the same workflow with `extract`, `outline`, `apply --layout`, and `write`.

**Tech Stack:** Go 1.25, standard library `archive/zip`, `encoding/json`, root package `github.com/kiry163/docxtidy`, existing `internal/ooxml` helpers.

---

## File Structure

- Modify `types.go`: rename public workflow types and remove non-core exported types.
- Modify `docxtidy.go`: update extract/write conversion helpers for `Snapshot`, `PackageSnapshot`, `SnapshotBlock`, and hidden numbering internals.
- Rename/modify `view.go`: keep file or rename to `outline.go`; expose `OutlineOf` and compact outline projection.
- Modify `layout.go`: replace role-based `Structure` and `Transform` validation/application with `Layout`, `Group`, and block-targeted `Edit`.
- Delete `repository.go`: remove the unused public persistence interface.
- Modify `docxtidy_test.go`: update API names and add layout/edit validation coverage.
- Modify `cmd/docxtidy/main.go`: rename `view` command to `outline`; change `apply` to accept `--layout`.
- Modify `cmd/docxtidy/main_test.go`: update CLI tests and fixtures to snapshot/outline/layout terminology.
- Modify `README.md`: update examples, CLI docs, and public workflow language.
- Leave `internal/ooxml/*` behavior intact except for already-existing numbering fixes.

## Task 1: Rename Core Types to Snapshot and Outline

**Files:**
- Modify: `types.go`
- Modify: `docxtidy.go`
- Modify: `view.go`
- Modify: `docxtidy_test.go`

- [ ] **Step 1: Write failing compile-level expectations in tests**

Update existing tests in `docxtidy_test.go` to use the new names:

```go
func TestExtractReturnsSnapshotFromReader(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{DocumentID: "sample"})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	if snapshot.Document.ID != "sample" {
		t.Fatalf("document id = %q, want sample", snapshot.Document.ID)
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
```

Rename `TestViewOfReturnsLightweightDocumentView` to:

```go
func TestOutlineOfReturnsLightweightDocumentOutline(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{DocumentID: "sample"})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	outline := OutlineOf(snapshot, OutlineOptions{})
	if outline.DocumentID != "sample" {
		t.Fatalf("outline document id = %q, want sample", outline.DocumentID)
	}
	if len(outline.Blocks) == 0 {
		t.Fatal("outline has no blocks")
	}
	if outline.Blocks[0].ID == "" {
		t.Fatal("first outline block ID is empty")
	}
}
```

Update helper names:

```go
func snapshotHasPackagePart(snapshot Snapshot, name string) bool {
	for _, part := range snapshot.Package.Parts {
		if part.Name == name {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify expected compile failure**

Run:

```bash
go test ./...
```

Expected: FAIL with missing identifiers such as `Snapshot`, `OutlineOf`, `OutlineOptions`, or `Package`.

- [ ] **Step 3: Update public type names in `types.go`**

Replace the old top-level public types with:

```go
type ExtractOptions struct {
	DocumentID string `json:"document_id,omitempty"`
}

type Snapshot struct {
	Document DocumentSnapshot `json:"document"`
	Package  PackageSnapshot  `json:"package"`
}

type PackageSnapshot struct {
	Parts []PackagePart `json:"parts"`
}

type PackagePart struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

type DocumentSnapshot struct {
	ID     string          `json:"id,omitempty"`
	Blocks []SnapshotBlock `json:"blocks"`
}

type SnapshotBlock struct {
	ID          string    `json:"id"`
	Type        BlockType `json:"type"`
	Text        string    `json:"text,omitempty"`
	DisplayText string    `json:"display_text,omitempty"`
	XML         string    `json:"xml"`
	Protected   bool      `json:"protected,omitempty"`
}

type BlockType string

const (
	BlockTypeParagraph BlockType = "paragraph"
	BlockTypeTable     BlockType = "table"
	BlockTypeSection   BlockType = "sectPr"
)

type OutlineOptions struct{}

type Outline struct {
	DocumentID string         `json:"document_id,omitempty"`
	Blocks     []OutlineBlock `json:"blocks"`
}

type OutlineBlock struct {
	ID        string    `json:"id"`
	Index     int       `json:"index"`
	Type      BlockType `json:"type"`
	Text      string    `json:"text"`
	Protected bool      `json:"protected,omitempty"`
}
```

Do not keep exported `State`, `Document`, `Block`, `PackageFile`, `View`, `ViewBlock`, `ViewOptions`, `ViewTextMode`, `NumberingInfo`, or `NumberingKind` aliases in the root package.

- [ ] **Step 4: Update conversion helpers in `docxtidy.go`**

Change signatures:

```go
func Extract(ctx context.Context, r io.Reader, opts ExtractOptions) (Snapshot, error)
func Write(ctx context.Context, snapshot Snapshot, w io.Writer) error
```

Build `Snapshot` from OOXML:

```go
return Snapshot{
	Document: DocumentSnapshot{
		ID:     opts.DocumentID,
		Blocks: blocksFromOOXML(rawState.Blocks),
	},
	Package: PackageSnapshot{
		Parts: packagePartsFromOOXML(rawState.Files),
	},
}, nil
```

Use `SnapshotBlock` in block conversion. Keep numbering conversion private by discarding public numbering metadata:

```go
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
```

Convert back to `ooxml.Block` with `Numbering: nil`; write-back depends on XML, not public numbering fields.

- [ ] **Step 5: Update outline projection**

Rename `ViewOf` to:

```go
func OutlineOf(snapshot Snapshot, opts OutlineOptions) Outline
```

Use `snapshot.Document.Blocks`. Implement `outlineBlockText(block SnapshotBlock) string` using existing display behavior:

```go
if block.Type == BlockTypeTable {
	if markdown := ooxml.MarkdownTable(block.XML); markdown != "" {
		return markdown
	}
}
if block.DisplayText != "" {
	return block.DisplayText
}
return block.Text
```

- [ ] **Step 6: Run tests for core rename**

Run:

```bash
go test ./...
```

Expected: FAIL only in places still referencing the old layout/CLI concepts. Fix remaining old core names in root tests until core extract/write/outline tests compile.

## Task 2: Replace Structure and Transform with Layout

**Files:**
- Modify: `types.go`
- Modify: `layout.go`
- Modify: `docxtidy_test.go`

- [ ] **Step 1: Add failing layout reorder test**

Replace the old `TestApplyReturnsNewStateAndPreservesInput` with:

```go
func TestApplyReturnsNewSnapshotFromLayoutAndPreservesInput(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{DocumentID: "sample"})
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
		t.Fatalf("original abstract text changed to %q, want %q", snapshot.Document.Blocks[9].Text)
	}
}
```

Add helper:

```go
func standardTestLayout() Layout {
	return Layout{
		Groups: []Group{
			{BlockIDs: []string{"block-0004"}},
			{BlockIDs: []string{"block-0006", "block-0007"}},
			{BlockIDs: []string{"block-0008"}},
			{BlockIDs: []string{"block-0010"}},
			{BlockIDs: []string{"block-0011"}},
			{BlockIDs: []string{"block-0012", "block-0013", "block-0014", "block-0015", "block-0016", "block-0017", "block-0018", "block-0019", "block-0020", "block-0021", "block-0022", "block-0023", "block-0024", "block-0025", "block-0026", "block-0027", "block-0028", "block-0029", "block-0030", "block-0031", "block-0032", "block-0033", "block-0034", "block-0035", "block-0036", "block-0037", "block-0038", "block-0039", "block-0040", "block-0041", "block-0042", "block-0043", "block-0044", "block-0045", "block-0046"}},
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
```

- [ ] **Step 2: Add failing validation tests**

Add tests:

```go
func TestApplyRejectsMissingBlockInLayout(t *testing.T) {
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{})
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
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{})
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
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{})
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
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{})
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
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{})
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
	snapshot, err := Extract(context.Background(), bytes.NewReader(sampleDocx(t)), ExtractOptions{})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	layout := completeLayoutFromSnapshot(snapshot)
	layout.Edits = []Edit{{BlockID: "block-0010"}}

	if _, err := Apply(context.Background(), snapshot, layout); err == nil || !strings.Contains(err.Error(), "missing replacement") {
		t.Fatalf("Apply error = %v, want missing replacement error", err)
	}
}
```

Add helper:

```go
func completeLayoutFromSnapshot(snapshot Snapshot) Layout {
	blockIDs := make([]string, 0, len(snapshot.Document.Blocks))
	for _, block := range snapshot.Document.Blocks {
		blockIDs = append(blockIDs, block.ID)
	}
	return Layout{Groups: []Group{{BlockIDs: blockIDs}}}
}
```

- [ ] **Step 3: Run tests to verify expected failure**

Run:

```bash
go test ./...
```

Expected: FAIL with missing identifiers `Layout`, `Group`, `Edit`, `TextReplacement`, or old `Apply` signature mismatch.

- [ ] **Step 4: Add layout types**

Append to `types.go`:

```go
type Layout struct {
	Groups []Group `json:"groups"`
	Edits  []Edit  `json:"edits,omitempty"`
}

type Group struct {
	BlockIDs []string `json:"block_ids"`
}

type Edit struct {
	BlockID string           `json:"block_id"`
	Replace *TextReplacement `json:"replace,omitempty"`
}

type TextReplacement struct {
	Old string `json:"old"`
	New string `json:"new"`
}
```

- [ ] **Step 5: Rewrite `layout.go` signatures and validation**

Change signatures:

```go
func Validate(ctx context.Context, snapshot Snapshot, layout Layout) error
func Apply(ctx context.Context, snapshot Snapshot, layout Layout) (Snapshot, error)
```

Implement validation:

```go
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
```

- [ ] **Step 6: Rewrite apply logic**

Apply edits before reorder:

```go
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
```

Keep section protection:

```go
if hasBlockType(snapshot.Document.Blocks, BlockTypeSection) && rebuiltBlocks[len(rebuiltBlocks)-1].Type != BlockTypeSection {
	return Snapshot{}, fmt.Errorf("sectPr block must remain last")
}
```

Replace role edit helper with:

```go
func applyBlockTextEdit(blockByID map[string]SnapshotBlock, edit Edit) error {
	block := blockByID[edit.BlockID]
	if !strings.Contains(block.Text, edit.Replace.Old) {
		return fmt.Errorf("text edit cannot find %q in block %s", edit.Replace.Old, edit.BlockID)
	}
	updatedXML, err := ooxml.ReplaceFirstTextInBlockXML(block.XML, edit.Replace.Old, edit.Replace.New)
	if err != nil {
		return fmt.Errorf("replace text in %s: %w", edit.BlockID, err)
	}
	block.Text = strings.Replace(block.Text, edit.Replace.Old, edit.Replace.New, 1)
	if block.DisplayText != "" {
		block.DisplayText = strings.Replace(block.DisplayText, edit.Replace.Old, edit.Replace.New, 1)
	}
	block.XML = updatedXML
	blockByID[edit.BlockID] = block
	return nil
}
```

- [ ] **Step 7: Run layout tests**

Run:

```bash
go test ./... -run 'TestApply'
```

Expected: PASS for root package apply tests; CLI may still fail until Task 3.

## Task 3: Update CLI to Snapshot, Outline, and Layout

**Files:**
- Modify: `cmd/docxtidy/main.go`
- Modify: `cmd/docxtidy/main_test.go`

- [ ] **Step 1: Update CLI tests to new terminology**

Rename and update tests:

```go
func TestCLIExtractWritesSnapshot(t *testing.T)
func TestCLIExtractWritesReadableXMLInJSON(t *testing.T)
func TestCLIOutlineWritesDocumentOutline(t *testing.T)
func TestCLIApplyWritesUpdatedSnapshot(t *testing.T)
func TestCLIWriteCreatesDocxFromSnapshot(t *testing.T)
```

Use paths:

```go
snapshotPath := filepath.Join(workDir, "snapshot.json")
outlinePath := filepath.Join(workDir, "outline.json")
layoutPath := filepath.Join(workDir, "layout.json")
updatedPath := filepath.Join(workDir, "updated-snapshot.json")
```

Call:

```go
run([]string{"outline", snapshotPath, "--out", outlinePath})
run([]string{"apply", snapshotPath, "--layout", layoutPath, "--out", updatedPath})
```

Change helper return types:

```go
func readMainTestSnapshot(t *testing.T, snapshotPath string) docxtidy.Snapshot
func readMainTestOutline(t *testing.T, outlinePath string) docxtidy.Outline
func mainTestLayout() docxtidy.Layout
```

Use `snapshot.Package.Parts`, `outline.Blocks`, and `snapshot.Document.Blocks`.

- [ ] **Step 2: Run CLI tests to verify failure**

Run:

```bash
go test ./cmd/docxtidy
```

Expected: FAIL because `outline` command and `--layout` option are not implemented.

- [ ] **Step 3: Update CLI command dispatch and usage**

In `run`, replace `"view"` with `"outline"`:

```go
case "outline":
	return runOutline(args[1:])
```

Update usage:

```go
fmt.Fprintln(os.Stderr, "  docxtidy extract <input.docx> --out <snapshot.json>")
fmt.Fprintln(os.Stderr, "  docxtidy outline <snapshot.json> --out <outline.json>")
fmt.Fprintln(os.Stderr, "  docxtidy apply <snapshot.json> --layout <layout.json> --out <updated-snapshot.json>")
fmt.Fprintln(os.Stderr, "  docxtidy write <snapshot.json> --out <output.docx>")
```

- [ ] **Step 4: Update CLI handlers**

Rename `runView` to `runOutline` and use:

```go
var snapshot docxtidy.Snapshot
if err := readJSON(snapshotPath, &snapshot); err != nil {
	return fmt.Errorf("read snapshot: %w", err)
}

outline := docxtidy.OutlineOf(snapshot, docxtidy.OutlineOptions{})
```

Update `runApply`:

```go
layoutPath := options["layout"]
outPath := options["out"]
if snapshotPath == "" || layoutPath == "" || outPath == "" {
	return fmt.Errorf("usage: docxtidy apply <snapshot.json> --layout <layout.json> --out <updated-snapshot.json>")
}

var snapshot docxtidy.Snapshot
if err := readJSON(snapshotPath, &snapshot); err != nil {
	return fmt.Errorf("read snapshot: %w", err)
}
var layout docxtidy.Layout
if err := readJSON(layoutPath, &layout); err != nil {
	return fmt.Errorf("read layout: %w", err)
}

updated, err := docxtidy.Apply(context.Background(), snapshot, layout)
```

Update `runWrite` to read `docxtidy.Snapshot`.

- [ ] **Step 5: Run CLI tests**

Run:

```bash
go test ./cmd/docxtidy
```

Expected: PASS.

## Task 4: Remove Non-Core Public API and Update Docs

**Files:**
- Delete: `repository.go`
- Modify: `README.md`
- Modify: `docxtidy_test.go`
- Modify: `cmd/docxtidy/main_test.go`

- [ ] **Step 1: Delete repository interface**

Delete `repository.go`. This removes the unused exported `Repository` type.

- [ ] **Step 2: Check exported root symbols**

Run:

```bash
rg -n '^type [A-Z]|^func [A-Z]|^const \(' *.go
```

Expected exported concepts are limited to:

- `Extract`, `OutlineOf`, `Apply`, `Validate`, `Write`
- `ExtractOptions`
- `Snapshot`, `DocumentSnapshot`, `SnapshotBlock`
- `PackageSnapshot`, `PackagePart`
- `BlockType` and its constants
- `OutlineOptions`, `Outline`, `OutlineBlock`
- `Layout`, `Group`, `Edit`, `TextReplacement`

No `State`, `View`, `Structure`, `Transform`, `Section`, `TextEdit`, `Repository`, `NumberingInfo`, or `NumberingKind`.

- [ ] **Step 3: Update README workflow example**

Replace the library usage example with:

```go
snapshot, err := docxtidy.Extract(ctx, input, docxtidy.ExtractOptions{
	DocumentID: "example",
})
if err != nil {
	panic(err)
}

outline := docxtidy.OutlineOf(snapshot, docxtidy.OutlineOptions{})
_ = outline

layout := docxtidy.Layout{
	Groups: []docxtidy.Group{
		{BlockIDs: []string{"block-0001"}},
		{BlockIDs: []string{"block-0002"}},
	},
}
_ = layout

output, err := os.Create("output.docx")
if err != nil {
	panic(err)
}
defer output.Close()

if err := docxtidy.Write(ctx, snapshot, output); err != nil {
	panic(err)
}
```

Add prose that `Snapshot` is the opaque write-back carrier and `Outline` is what should be sent to external models.

- [ ] **Step 4: Update CLI docs**

Replace old CLI commands with:

```bash
go run ./cmd/docxtidy extract input.docx --out snapshot.json
go run ./cmd/docxtidy outline snapshot.json --out outline.json
go run ./cmd/docxtidy apply snapshot.json --layout layout.json --out updated-snapshot.json
go run ./cmd/docxtidy write updated-snapshot.json --out output.docx
```

- [ ] **Step 5: Run full verification**

Run:

```bash
gofmt -w *.go cmd/docxtidy/*.go
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Inspect final diff**

Run:

```bash
git diff --stat
git diff -- types.go docxtidy.go view.go layout.go cmd/docxtidy/main.go README.md
```

Expected: diff matches the spec and does not include unrelated changes besides the previously existing numbering fix if still present in the worktree.

## Self-Review

- Spec coverage: The plan covers workflow naming, public types, removed concepts, layout semantics, edit semantics, outline semantics, CLI JSON workflow, migration scope, and tests.
- Placeholder scan: No `TBD`, `TODO`, vague "add tests", or undefined future-only tasks remain.
- Type consistency: The plan consistently uses `Snapshot`, `Outline`, `Layout`, `Group`, `Edit`, and `TextReplacement`.
