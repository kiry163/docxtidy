package docxtidy

type ExtractOptions struct {
	DocumentID string
}

type DocumentState struct {
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

type Layout struct {
	Order            []string          `json:"order"`
	TextReplacements []TextReplacement `json:"text_replacements,omitempty"`
}

type TextReplacement struct {
	Role string `json:"role"`
	Old  string `json:"old"`
	New  string `json:"new"`
}
