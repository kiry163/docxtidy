package docxtidy

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
