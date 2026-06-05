package docxtidy

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

const documentXMLPath = "word/document.xml"

func Extract(ctx context.Context, r io.Reader, opts ExtractOptions) (DocumentState, error) {
	if err := ctx.Err(); err != nil {
		return DocumentState{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return DocumentState{}, fmt.Errorf("read docx: %w", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return DocumentState{}, fmt.Errorf("open docx: %w", err)
	}

	var files []PackageFile
	var documentXML []byte
	var numberingXML []byte
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return DocumentState{}, err
		}
		if file.FileInfo().IsDir() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return DocumentState{}, fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}
		fileData, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return DocumentState{}, fmt.Errorf("read zip entry %s: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return DocumentState{}, fmt.Errorf("close zip entry %s: %w", file.Name, closeErr)
		}
		files = append(files, PackageFile{Name: file.Name, Data: fileData})
		if file.Name == documentXMLPath {
			documentXML = fileData
		}
		if file.Name == "word/numbering.xml" {
			numberingXML = fileData
		}
	}
	if len(documentXML) == 0 {
		return DocumentState{}, fmt.Errorf("docx missing %s", documentXMLPath)
	}

	numberingDefinitions, err := parseNumberingDefinitions(numberingXML)
	if err != nil {
		return DocumentState{}, err
	}
	blocks, err := bodyBlocks(documentXML, numberingDefinitions)
	if err != nil {
		return DocumentState{}, err
	}

	return DocumentState{
		Document: Document{
			ID:     opts.DocumentID,
			Blocks: blocks,
		},
		Files: files,
	}, nil
}

func Write(ctx context.Context, state DocumentState, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	writer := zip.NewWriter(w)
	defer writer.Close()

	files := append([]PackageFile(nil), state.Files...)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	foundDocumentXML := false
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		data := file.Data
		if file.Name == documentXMLPath {
			foundDocumentXML = true
			rebuilt, err := replaceBodyBlocks(data, state.Document.Blocks)
			if err != nil {
				return err
			}
			data = rebuilt
		}

		header := &zip.FileHeader{
			Name:   file.Name,
			Method: zip.Deflate,
		}
		entryWriter, err := writer.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create zip entry %s: %w", file.Name, err)
		}
		if _, err := entryWriter.Write(data); err != nil {
			return fmt.Errorf("write zip entry %s: %w", file.Name, err)
		}
	}
	if !foundDocumentXML {
		return fmt.Errorf("state missing %s", documentXMLPath)
	}
	return nil
}

func bodyBlocks(documentXML []byte, numberingDefinitions numberingDefinitions) ([]Block, error) {
	decoder := xml.NewDecoder(bytes.NewReader(documentXML))
	var blocks []Block
	inBody := false
	depth := 0
	blockStart := -1
	blockType := ""
	numberingState := newNumberingState(numberingDefinitions)

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", documentXMLPath, err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			tokenEnd := int(decoder.InputOffset())
			tokenStart := startTagOffset(documentXML, tokenEnd)
			if !inBody && t.Name.Local == "body" {
				inBody = true
				depth = 0
				continue
			}
			if inBody {
				if depth == 0 {
					blockStart = tokenStart
					blockType = blockTypeFromName(t.Name.Local)
				}
				depth++
			}
		case xml.EndElement:
			tokenEnd := int(decoder.InputOffset())
			if !inBody {
				continue
			}
			if depth == 0 {
				if t.Name.Local == "body" {
					inBody = false
				}
				continue
			}

			depth--
			if depth == 0 && blockStart >= 0 {
				raw := string(documentXML[blockStart:tokenEnd])
				text := blockText([]byte(raw))
				numbering := numberingState.numberingInfoForBlock(raw)
				displayText := text
				if numbering != nil && numbering.ComputedLabel != "" {
					displayText = numbering.ComputedLabel + " " + text
				}
				blocks = append(blocks, Block{
					ID:          fmt.Sprintf("block-%04d", len(blocks)+1),
					Type:        BlockType(blockType),
					Text:        text,
					DisplayText: displayText,
					XML:         raw,
					Numbering:   numbering,
					Protected:   blockType == string(BlockTypeSection),
				})
				blockStart = -1
				blockType = ""
			}
		}
	}

	if !inBody && blocks != nil {
		return blocks, nil
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("%s contains no body blocks", documentXMLPath)
	}
	return blocks, nil
}

func startTagOffset(data []byte, tokenEnd int) int {
	if tokenEnd > len(data) {
		tokenEnd = len(data)
	}
	for i := tokenEnd - 1; i >= 0; i-- {
		if data[i] == '<' {
			return i
		}
	}
	return 0
}

func blockTypeFromName(local string) string {
	switch local {
	case "p":
		return string(BlockTypeParagraph)
	case "tbl":
		return string(BlockTypeTable)
	default:
		return local
	}
}

func blockText(blockXML []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(blockXML))
	var builder strings.Builder
	textDepth := 0

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				textDepth++
			}
		case xml.EndElement:
			if t.Name.Local == "t" && textDepth > 0 {
				textDepth--
			}
		case xml.CharData:
			if textDepth > 0 {
				builder.Write([]byte(t))
			}
		}
	}

	return builder.String()
}

type numberingDefinitions struct {
	numToAbstract map[string]string
	levels        map[string]map[int]string
}

func parseNumberingDefinitions(numberingXML []byte) (numberingDefinitions, error) {
	definitions := numberingDefinitions{
		numToAbstract: map[string]string{},
		levels:        map[string]map[int]string{},
	}
	if len(numberingXML) == 0 {
		return definitions, nil
	}

	decoder := xml.NewDecoder(bytes.NewReader(numberingXML))
	currentAbstractID := ""
	currentLevel := -1
	currentNumID := ""
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return numberingDefinitions{}, fmt.Errorf("parse word/numbering.xml: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "abstractNum":
				currentAbstractID = attrValue(t, "abstractNumId")
				if currentAbstractID != "" && definitions.levels[currentAbstractID] == nil {
					definitions.levels[currentAbstractID] = map[int]string{}
				}
			case "lvl":
				currentLevel = attrInt(t, "ilvl", -1)
			case "lvlText":
				if currentAbstractID != "" && currentLevel >= 0 {
					definitions.levels[currentAbstractID][currentLevel] = attrValue(t, "val")
				}
			case "num":
				currentNumID = attrValue(t, "numId")
			case "abstractNumId":
				if currentNumID != "" {
					definitions.numToAbstract[currentNumID] = attrValue(t, "val")
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "abstractNum":
				currentAbstractID = ""
				currentLevel = -1
			case "lvl":
				currentLevel = -1
			case "num":
				currentNumID = ""
			}
		}
	}
	return definitions, nil
}

type numberingState struct {
	definitions numberingDefinitions
	counters    map[string][]int
}

func newNumberingState(definitions numberingDefinitions) *numberingState {
	return &numberingState{
		definitions: definitions,
		counters:    map[string][]int{},
	}
}

func (s *numberingState) numberingInfoForBlock(blockXML string) *NumberingInfo {
	numID, level, ok := paragraphNumbering(blockXML)
	if !ok {
		return nil
	}

	counters := s.counters[numID]
	if len(counters) < 9 {
		counters = make([]int, 9)
	}
	for i := 0; i < level; i++ {
		if counters[i] == 0 {
			counters[i] = 1
		}
	}
	counters[level]++
	for i := level + 1; i < len(counters); i++ {
		counters[i] = 0
	}
	s.counters[numID] = counters

	levelText := s.levelText(numID, level)
	computedLabel := computeNumberingLabel(levelText, counters)
	return &NumberingInfo{
		Kind:          NumberingKindAuto,
		NumID:         numID,
		Level:         level,
		LevelText:     levelText,
		ComputedLabel: computedLabel,
	}
}

func (s *numberingState) levelText(numID string, level int) string {
	abstractID := s.definitions.numToAbstract[numID]
	if abstractID == "" {
		return ""
	}
	return s.definitions.levels[abstractID][level]
}

func paragraphNumbering(blockXML string) (string, int, bool) {
	decoder := xml.NewDecoder(strings.NewReader(blockXML))
	inPPr := false
	inNumPr := false
	numID := ""
	level := -1
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pPr":
				inPPr = true
			case "numPr":
				if inPPr {
					inNumPr = true
				}
			case "numId":
				if inNumPr {
					numID = attrValue(t, "val")
				}
			case "ilvl":
				if inNumPr {
					level = attrInt(t, "val", -1)
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "numPr":
				inNumPr = false
			case "pPr":
				inPPr = false
			}
		}
	}
	return numID, level, numID != "" && level >= 0
}

func computeNumberingLabel(levelText string, counters []int) string {
	if levelText == "" {
		return ""
	}
	label := levelText
	for i := 0; i < len(counters); i++ {
		placeholder := fmt.Sprintf("%%%d", i+1)
		if strings.Contains(label, placeholder) {
			label = strings.ReplaceAll(label, placeholder, fmt.Sprintf("%d", counters[i]))
		}
	}
	return label
}

func attrValue(element xml.StartElement, local string) string {
	for _, attr := range element.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func attrInt(element xml.StartElement, local string, fallback int) int {
	value := attrValue(element, local)
	if value == "" {
		return fallback
	}
	result := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return fallback
		}
		result = result*10 + int(ch-'0')
	}
	return result
}

func replaceBodyBlocks(documentXML []byte, blocks []Block) ([]byte, error) {
	contentStart, contentEnd, err := bodyContentOffsets(documentXML)
	if err != nil {
		return nil, err
	}

	var builder strings.Builder
	for _, block := range blocks {
		if block.XML == "" {
			return nil, fmt.Errorf("block %s has empty xml", block.ID)
		}
		builder.WriteString(block.XML)
	}

	result := make([]byte, 0, contentStart+builder.Len()+len(documentXML)-contentEnd)
	result = append(result, documentXML[:contentStart]...)
	result = append(result, builder.String()...)
	result = append(result, documentXML[contentEnd:]...)
	return result, nil
}

func bodyContentOffsets(documentXML []byte) (int, int, error) {
	decoder := xml.NewDecoder(bytes.NewReader(documentXML))
	inBody := false
	depth := 0
	contentStart := -1

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("parse %s: %w", documentXMLPath, err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			if !inBody && t.Name.Local == "body" {
				inBody = true
				depth = 0
				contentStart = int(decoder.InputOffset())
				continue
			}
			if inBody {
				depth++
			}
		case xml.EndElement:
			if !inBody {
				continue
			}
			if depth == 0 && t.Name.Local == "body" {
				return contentStart, startTagOffset(documentXML, int(decoder.InputOffset())), nil
			}
			if depth > 0 {
				depth--
			}
		}
	}

	return 0, 0, fmt.Errorf("%s missing body element", documentXMLPath)
}
