# DocxTidy Library Design

## Goal

Refactor the current DOCX layout validation prototype into a Go library named `DocxTidy` with module path `github.com/kiry163/docxtidy`.

DocxTidy provides high-fidelity DOCX extraction, structure-friendly document views, deterministic layout application, validation, and DOCX rebuilding. It does not call or wrap any LLM. Users may send the extracted view to an LLM, rules engine, or manual process, then pass the resulting structure and layout data back to DocxTidy.

## Product Positioning

DocxTidy is a Go library first. The CLI is only an example and debugging tool.

Primary library responsibilities:

- Read a `.docx` from an `io.Reader`.
- Extract a re-buildable intermediate state.
- Produce block-level text and metadata suitable for external structure recognition.
- Accept user-provided structure and layout values.
- Validate that layout operations do not drop, duplicate, or corrupt blocks.
- Generate a new `.docx` through an `io.Writer`.

Explicit non-goals:

- Built-in LLM calls.
- Built-in prompt orchestration.
- Built-in business document schemas.
- Full visual rendering parity with Word/WPS.
- A mandatory storage backend.

## Module And Package Shape

Module path:

```text
github.com/kiry163/docxtidy
```

Recommended package layout:

```text
.
├── docxtidy.go
├── layout.go
├── types.go
├── repository.go
├── internal/ooxml/
└── cmd/docxtidy/
```

`docxtidy` is the public library package. OOXML package parsing, document body parsing, display text generation, Markdown table projection, and XML text replacement live in `internal/ooxml` to keep implementation details out of the public module contract. `cmd/docxtidy` wraps the public API for manual debugging and examples.

## Core API

The core API should operate directly on `State`. This keeps the library storage-agnostic and easy to test.

```go
func Extract(ctx context.Context, r io.Reader, opts ExtractOptions) (State, error)

func ViewOf(state State, opts ViewOptions) View

func Apply(ctx context.Context, state State, structure Structure, transform Transform) (State, error)

func Validate(ctx context.Context, state State, structure Structure, transform Transform) error

func Write(ctx context.Context, state State, w io.Writer) error
```

`Extract` reads and unpacks a DOCX into a re-buildable state. `ViewOf` returns a lightweight, reader-oriented projection for user review or external structure recognition. `Apply` returns a new state rather than mutating the input. `Validate` exposes safety checks without rebuilding. `Write` emits the final DOCX to an `io.Writer`.

## Optional Repository Interface

DocxTidy should define a high-level repository interface for users who want persistence, but the core API must not require it.

```go
type Repository interface {
    Save(ctx context.Context, docID string, state State) error
    Load(ctx context.Context, docID string) (State, error)
    Delete(ctx context.Context, docID string) error
}
```

The repository stores a whole DocxTidy intermediate state. Users can implement local files, object storage, databases, or their own business storage. A future package may provide example repositories, but the public core remains independent.

## Data Model

```go
type State struct {
    Document Document
    Files    []PackageFile
}

type PackageFile struct {
    Name string
    Data []byte
}

type Document struct {
    ID     string
    Blocks []Block
}

type Block struct {
    ID          string
    Type        BlockType
    Text        string
    DisplayText string
    XML         string
    Numbering   *NumberingInfo
    Protected   bool
}

type NumberingInfo struct {
    Kind          NumberingKind
    NumID         string
    Level         int
    LevelText     string
    ComputedLabel string
}

type Structure struct {
    Sections []Section
}

type Section struct {
    Role     string
    BlockIDs []string
}

type ViewOptions struct {
    TextMode ViewTextMode
}

type ViewTextMode string

const (
    ViewTextModeDisplay ViewTextMode = "display"
    ViewTextModeSource  ViewTextMode = "source"
)

type View struct {
    DocumentID string
    Blocks []ViewBlock
}

type ViewBlock struct {
    ID          string
    Index       int
    Type        BlockType
    Text        string
    Protected   bool
}

type Transform struct {
    Order            []string
    TextEdits        []TextEdit
}

type TextEdit struct {
    Role string
    Old  string
    New  string
}
```

`Block.Text` is the text physically present in OOXML text nodes. `Block.DisplayText` is a best-effort reader-oriented string that may include computed numbering labels and Markdown image placeholders such as `![图片 1](media:rId5)`. Image-only paragraphs therefore keep `Text` empty while `DisplayText` shows the placeholder. `Block.Type` reflects the OOXML body container, so a paragraph containing a drawing remains a paragraph. `ViewBlock.Text` uses display text by default and does not expose numbering internals. In display mode, table blocks are projected as best-effort Markdown tables for readability, including image placeholders inside table cells; source mode keeps the original extracted text. Set `ViewOptions.TextMode` to `ViewTextModeSource` when callers need source text instead. Mutation should target source blocks and raw XML, not the view.

## Numbering

DocxTidy should distinguish hand-written numbering from OOXML automatic numbering.

- Hand-written numbering appears inside `w:t` and is part of `Text`.
- Automatic numbering appears through paragraph numbering properties such as `w:pPr/w:numPr`, or through inherited paragraph styles.
- Blocks with automatic numbering should populate `NumberingInfo` and `DisplayText`.

Automatic numbering is layout-sensitive. Moving numbered blocks may change displayed labels in Word/WPS. DocxTidy should expose that fact but not attempt full layout rendering.

## Safety Rules

Transform application must fail when:

- A structure section references an unknown block ID.
- A block appears in more than one section.
- A block from the source document is missing.
- Transform order references an unknown role.
- Transform order omits a role present in the structure.
- A protected block would be dropped or moved into an invalid location.
- A text replacement cannot find its target text inside the target role.

The initial protected block rule should keep section properties (`sectPr`) last. Future versions may protect bookmarks, drawing anchors, comment ranges, and relationship-sensitive blocks more precisely.

## CLI Role

The CLI remains useful but secondary:

```bash
docxtidy extract input.docx --out state.json
docxtidy view state.json --out view.json
docxtidy apply state.json --structure structure.json --transform transform.json --out new-state.json
docxtidy write new-state.json --out output.docx
```

The CLI should call the public API rather than internal packages. It exists for manual verification and examples.

## Migration Status

The prototype has been migrated to the library-first shape:

- The module is `github.com/kiry163/docxtidy`.
- Public types and functions live in package `docxtidy`.
- The old path-based `internal/docxrt` prototype has been removed.
- Core operations use reader/state/writer APIs.
- `cmd/docxtidy` is a thin wrapper around the public API.
- API-level tests cover `Extract`, `ViewOf`, `Apply`, numbering metadata, and `Write`; CLI tests cover the state/view/transform JSON workflow.

## V1 Decisions

- `State.Files []PackageFile` is acceptable for v1. Streaming state can be added later if large files become a practical bottleneck.
- V1 defines the `Repository` interface but does not need built-in repository implementations beyond tests or examples.
- V1 numbering should expose direct paragraph numbering metadata and compute simple labels when possible. Full Word/WPS rendering parity is out of scope.

## Notes

Tests should generate minimal DOCX fixtures at runtime. The repository should not commit Word files or require Word files as test inputs.
