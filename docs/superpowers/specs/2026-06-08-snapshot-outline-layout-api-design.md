# Snapshot, Outline, and Layout API Design

## Purpose

DocxTidy is a workflow library for DOCX restructuring:

1. Read a DOCX.
2. Extract a complete write-back snapshot.
3. Produce a compact, readable outline for a user or external model.
4. Let the user or model produce a target layout.
5. Apply the layout to the original snapshot.
6. Write the updated DOCX.

The public API should optimize for this pipeline only. It should not look like a
general-purpose DOCX editing library.

## Naming

Replace the broad `State` and `View` names with workflow-specific names:

- `Snapshot`: the complete extracted document package required for write-back.
- `Outline`: a readable model-facing projection of the snapshot.
- `Layout`: the target block arrangement and small explicit edits.

These names define the mental model:

- Save or persist `Snapshot`.
- Send `Outline` to a model or human reviewer.
- Receive or construct `Layout`.
- Apply `Layout` to `Snapshot`.

## Public Workflow

The primary usage should read as:

```go
snapshot, err := docxtidy.Extract(ctx, input)
if err != nil {
    return err
}

outline := docxtidy.OutlineOf(snapshot)

layout := docxtidy.Layout{
    Groups: []docxtidy.Group{
        {BlockIDs: []string{"block-0001", "block-0002"}},
        {BlockIDs: []string{"block-0003"}},
    },
    Edits: []docxtidy.Edit{
        {
            BlockID: "block-0002",
            Replace: &docxtidy.TextReplacement{
                Old: "old text",
                New: "new text",
            },
        },
    },
}

updated, err := docxtidy.Apply(ctx, snapshot, layout)
if err != nil {
    return err
}

if err := docxtidy.Write(ctx, updated, output); err != nil {
    return err
}
```

## Public Types

The root package should expose only concepts needed for the workflow.

```go
func Extract(ctx context.Context, r io.Reader) (Snapshot, error)
func OutlineOf(snapshot Snapshot) Outline
func Apply(ctx context.Context, snapshot Snapshot, layout Layout) (Snapshot, error)
func Validate(ctx context.Context, snapshot Snapshot, layout Layout) error
func Write(ctx context.Context, snapshot Snapshot, w io.Writer) error
```

Core data types:

```go
type Snapshot struct {
    Document DocumentSnapshot `json:"document"`
    Package  PackageSnapshot  `json:"package"`
}

type DocumentSnapshot struct {
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

type PackageSnapshot struct {
    Parts []PackagePart `json:"parts"`
}

type PackagePart struct {
    Name string `json:"name"`
    Data []byte `json:"data"`
}

type Outline struct {
    Blocks []OutlineBlock `json:"blocks"`
}

type OutlineBlock struct {
    ID        string    `json:"id"`
    Index     int       `json:"index"`
    Type      BlockType `json:"type"`
    Text      string    `json:"text"`
    Protected bool      `json:"protected,omitempty"`
}

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

`SnapshotBlock.XML`, `PackageSnapshot`, and `PackagePart` remain public because
the snapshot must be JSON-serializable and write-back capable. Documentation
must mark them as opaque implementation data: callers should preserve them
as-is unless they deliberately accept OOXML-level responsibility.

## Removed or Hidden Concepts

Remove these from the root public API:

- `Repository`: persistence is application-specific and not part of the core
  pipeline.
- `NumberingInfo` and `NumberingKind`: numbering is best-effort rendering
  metadata and should appear through display text instead of OOXML internals.
- `Structure`, `Section`, `Transform`, and role-based text edits: replace them
  with `Layout`, `Group`, and block-targeted edits.
- `View`, `ViewBlock`, `ViewOptions`, and `ViewTextMode`: replace them with
  `Outline` and `OutlineBlock`.

## Layout Semantics

DocxTidy does not interpret business roles such as title, abstract, body, or
references. The caller owns those semantics. The library only receives block
IDs and applies the requested arrangement.

`Layout.Groups` is a complete target arrangement:

- Group order is document order.
- Block order inside each group is preserved.
- Every source block must appear exactly once unless future APIs add an
  explicit delete operation.
- Unknown block IDs fail validation.
- Duplicate block IDs fail validation.
- Empty groups fail validation.
- Protected blocks must remain in legal positions. Initially this means a
  `sectPr` block must remain last.

This strict coverage rule avoids silent data loss or surprising auto-append
behavior.

## Edit Semantics

The first version supports only explicit text replacement:

- `Edit.BlockID` must reference an existing block.
- `Edit.Replace` must be present.
- `TextReplacement.Old` must be non-empty.
- The old text must be found in the target block source text.
- Protected blocks cannot be edited.
- Replacement updates both source text and underlying OOXML.

Edits run before block reordering. Future delete or insert support should use
explicit fields rather than overloading missing block IDs or empty groups.

## Outline Semantics

`Outline` is the model-facing representation. It should stay compact and
readable:

- It includes `ID`, `Index`, `Type`, `Text`, and `Protected`.
- `Text` uses display text by default, including computed numbering labels,
  Markdown table projections, and image placeholders when available.
- It does not expose raw XML, package data, numbering internals, or other
  write-back-only fields.

`Index` remains useful for debugging and model prompts, but layout application
uses only block IDs.

## JSON Workflow

The CLI should mirror the public API:

```bash
docxtidy extract input.docx --out snapshot.json
docxtidy outline snapshot.json --out outline.json
docxtidy apply snapshot.json --layout layout.json --out updated-snapshot.json
docxtidy write updated-snapshot.json --out output.docx
```

The CLI should no longer expose separate `structure` and `transform` inputs.

## Migration Scope

Because the library is pre-release and has no users, this can be a breaking
API cleanup. Tests and examples should be updated to the new names and flow.

Implementation should keep the internal OOXML package as the engine. The root
package remains the workflow API, not a general editing facade.

## Testing

Coverage should prove:

- Existing extraction and write round-trip behavior still works.
- `OutlineOf` omits opaque snapshot fields and keeps readable text.
- `Apply` accepts a complete layout and reorders blocks.
- `Apply` rejects missing, duplicate, unknown, and empty group block IDs.
- Protected section blocks remain last.
- Text replacement succeeds for matching source text and updates the written
  DOCX state.
- Text replacement fails for missing old text, unknown block IDs, missing
  replacement payloads, and protected blocks.
- CLI commands use `snapshot`, `outline`, and `layout` terminology.
