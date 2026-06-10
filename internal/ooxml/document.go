package ooxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

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
				text := BlockText(raw)
				numbering := numberingState.numberingInfoForBlock(raw)
				displayText := DisplayTextFromBlockXML(raw, text)
				if numbering != nil && numbering.ComputedLabel != "" {
					displayText = numbering.ComputedLabel + displayText
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

func BlockText(blockXML string) string {
	decoder := xml.NewDecoder(strings.NewReader(blockXML))
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
