package ooxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type textRange struct {
	start int
	end   int
	text  string
}

func ReplaceFirstTextInBlockXML(blockXML string, oldText string, newText string) (string, error) {
	ranges, fullText, err := textRangesInBlockXML(blockXML)
	if err != nil {
		return "", err
	}
	if len(ranges) == 0 {
		return "", fmt.Errorf("block has no text nodes")
	}

	replaceStart := strings.Index(fullText, oldText)
	if replaceStart == -1 {
		return "", fmt.Errorf("text %q not found", oldText)
	}
	replaceEnd := replaceStart + len(oldText)

	affectedStart := -1
	affectedEnd := -1
	cursor := 0
	rangeStarts := make([]int, len(ranges))
	rangeEnds := make([]int, len(ranges))
	for i, textRange := range ranges {
		rangeStarts[i] = cursor
		cursor += len(textRange.text)
		rangeEnds[i] = cursor
		if rangeEnds[i] > replaceStart && rangeStarts[i] < replaceEnd {
			if affectedStart == -1 {
				affectedStart = i
			}
			affectedEnd = i
		}
	}
	if affectedStart == -1 {
		return "", fmt.Errorf("text %q did not overlap text nodes", oldText)
	}

	replacements := make(map[int]string)
	first := ranges[affectedStart]
	firstPrefix := first.text[:replaceStart-rangeStarts[affectedStart]]
	if affectedStart == affectedEnd {
		firstSuffix := first.text[replaceEnd-rangeStarts[affectedStart]:]
		replacements[affectedStart] = firstPrefix + newText + firstSuffix
	} else {
		last := ranges[affectedEnd]
		lastSuffix := last.text[replaceEnd-rangeStarts[affectedEnd]:]
		replacements[affectedStart] = firstPrefix + newText
		for i := affectedStart + 1; i < affectedEnd; i++ {
			replacements[i] = ""
		}
		replacements[affectedEnd] = lastSuffix
	}

	updated := []byte(blockXML)
	for i := len(ranges) - 1; i >= 0; i-- {
		replacement, ok := replacements[i]
		if !ok {
			continue
		}
		escaped := escapeXMLText(replacement)
		textRange := ranges[i]
		updated = append(updated[:textRange.start], append([]byte(escaped), updated[textRange.end:]...)...)
	}

	return string(updated), nil
}

func textRangesInBlockXML(blockXML string) ([]textRange, string, error) {
	data := []byte(blockXML)
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var ranges []textRange
	var fullText strings.Builder
	textStart := -1

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("parse block xml: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				textStart = int(decoder.InputOffset())
			}
		case xml.EndElement:
			if t.Name.Local == "t" && textStart >= 0 {
				textEnd := startTagOffset(data, int(decoder.InputOffset()))
				raw := string(data[textStart:textEnd])
				decoded := decodeXMLText(raw)
				ranges = append(ranges, textRange{
					start: textStart,
					end:   textEnd,
					text:  decoded,
				})
				fullText.WriteString(decoded)
				textStart = -1
			}
		}
	}

	return ranges, fullText.String(), nil
}

func decodeXMLText(raw string) string {
	decoder := xml.NewDecoder(strings.NewReader("<x>" + raw + "</x>"))
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if charData, ok := token.(xml.CharData); ok {
			builder.Write([]byte(charData))
		}
	}
	return builder.String()
}

func escapeXMLText(text string) string {
	var builder strings.Builder
	if err := xml.EscapeText(&builder, []byte(text)); err != nil {
		return text
	}
	return builder.String()
}
