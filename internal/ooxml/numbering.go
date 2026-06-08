package ooxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type numberingDefinitions struct {
	numToAbstract map[string]string
	levels        map[string]map[int]string
	starts        map[string]map[int]int
}

func parseNumberingDefinitions(numberingXML []byte) (numberingDefinitions, error) {
	definitions := numberingDefinitions{
		numToAbstract: map[string]string{},
		levels:        map[string]map[int]string{},
		starts:        map[string]map[int]int{},
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
				if currentAbstractID != "" && definitions.starts[currentAbstractID] == nil {
					definitions.starts[currentAbstractID] = map[int]int{}
				}
			case "lvl":
				currentLevel = attrInt(t, "ilvl", -1)
			case "start":
				if currentAbstractID != "" && currentLevel >= 0 {
					definitions.starts[currentAbstractID][currentLevel] = attrInt(t, "val", 1)
				}
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
			counters[i] = s.levelStart(numID, i)
		}
	}
	if counters[level] == 0 {
		counters[level] = s.levelStart(numID, level)
	} else {
		counters[level]++
	}
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

func (s *numberingState) levelStart(numID string, level int) int {
	abstractID := s.definitions.numToAbstract[numID]
	if abstractID == "" {
		return 1
	}
	start := s.definitions.starts[abstractID][level]
	if start == 0 {
		return 1
	}
	return start
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

func RebuildManualNumberingParagraphXML(blockXML string, text string, style string) (string, error) {
	startTagEnd := strings.Index(blockXML, ">")
	if startTagEnd == -1 || !strings.HasPrefix(blockXML, "<w:p") {
		return "", fmt.Errorf("manual numbering edit requires paragraph XML")
	}

	startTag := blockXML[:startTagEnd+1]
	return startTag + manualNumberingParagraphProperties(style) + manualNumberingRunXML(text, style) + "</w:p>", nil
}

func manualNumberingParagraphProperties(style string) string {
	switch style {
	case "heading":
		return `<w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>`
	default:
		return `<w:pPr/>`
	}
}

func manualNumberingRunXML(text string, style string) string {
	rPr := ""
	if style == "heading" {
		rPr = `<w:rPr><w:b/></w:rPr>`
	}
	return `<w:r>` + rPr + manualNumberingTextXML(text) + `</w:r>`
}

func manualNumberingTextXML(text string) string {
	space := ""
	if strings.TrimSpace(text) != text {
		space = ` xml:space="preserve"`
	}
	return `<w:t` + space + `>` + escapeXMLText(text) + `</w:t>`
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
