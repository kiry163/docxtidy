package docxtidy

type ExtractOptions struct {
	DocumentID string
}

type State struct {
	Document Document      `json:"document"`
	Files    []PackageFile `json:"files"`
}

type PackageFile struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

type Document struct {
	ID     string  `json:"id"`
	Blocks []Block `json:"blocks"`
}

type Block struct {
	ID          string         `json:"id"`
	Type        BlockType      `json:"type"`
	Text        string         `json:"text"`
	DisplayText string         `json:"display_text,omitempty"`
	XML         string         `json:"xml"`
	Numbering   *NumberingInfo `json:"numbering,omitempty"`
	Protected   bool           `json:"protected,omitempty"`
}

type BlockType string

const (
	BlockTypeParagraph BlockType = "paragraph"
	BlockTypeTable     BlockType = "table"
	BlockTypeSection   BlockType = "sectPr"
)

type NumberingInfo struct {
	Kind          NumberingKind `json:"kind"`
	NumID         string        `json:"num_id"`
	Level         int           `json:"level"`
	LevelText     string        `json:"level_text,omitempty"`
	ComputedLabel string        `json:"computed_label,omitempty"`
}

type NumberingKind string

const (
	NumberingKindAuto    NumberingKind = "auto"
	NumberingKindWritten NumberingKind = "written"
)

type Structure struct {
	Sections []Section `json:"sections"`
}

type Section struct {
	Role     string   `json:"role"`
	BlockIDs []string `json:"block_ids"`
}

type Transform struct {
	Order     []string   `json:"order"`
	TextEdits []TextEdit `json:"text_edits,omitempty"`
}

type TextEdit struct {
	Role string `json:"role"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

type ViewOptions struct {
	TextMode ViewTextMode `json:"text_mode,omitempty"`
}

type ViewTextMode string

const (
	ViewTextModeDisplay ViewTextMode = "display"
	ViewTextModeSource  ViewTextMode = "source"
)

type View struct {
	DocumentID string      `json:"document_id,omitempty"`
	Blocks     []ViewBlock `json:"blocks"`
}

type ViewBlock struct {
	ID        string    `json:"id"`
	Index     int       `json:"index"`
	Type      BlockType `json:"type"`
	Text      string    `json:"text"`
	Protected bool      `json:"protected,omitempty"`
}
