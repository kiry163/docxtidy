package ooxml

type PackageFile struct {
	Name string
	Data []byte
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

type BlockType string

const (
	BlockTypeParagraph BlockType = "paragraph"
	BlockTypeTable     BlockType = "table"
	BlockTypeSection   BlockType = "sectPr"
)

type NumberingInfo struct {
	Kind          NumberingKind
	NumID         string
	Level         int
	LevelText     string
	ComputedLabel string
}

type NumberingKind string

const (
	NumberingKindAuto NumberingKind = "auto"
)

type State struct {
	Blocks []Block
	Files  []PackageFile
}
