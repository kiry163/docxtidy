package ooxml

import (
	"encoding/xml"
	"io"
	"strings"
)

func MarkdownTable(blockXML string) string {
	rows := tableRows(blockXML)
	if len(rows) == 0 {
		return ""
	}

	columnCount := 0
	for _, row := range rows {
		if len(row) > columnCount {
			columnCount = len(row)
		}
	}
	if columnCount == 0 {
		return ""
	}

	var builder strings.Builder
	writeMarkdownRow(&builder, rows[0], columnCount)
	separator := make([]string, columnCount)
	for i := range separator {
		separator[i] = "---"
	}
	writeMarkdownRow(&builder, separator, columnCount)
	for _, row := range rows[1:] {
		writeMarkdownRow(&builder, row, columnCount)
	}
	return strings.TrimRight(builder.String(), "\n")
}

func tableRows(blockXML string) [][]string {
	decoder := xml.NewDecoder(strings.NewReader(blockXML))
	var rows [][]string
	var row []string
	var cell strings.Builder
	inRow := false
	cellDepth := 0
	textDepth := 0
	drawingDepth := 0
	drawing := drawingImageRef{}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}

		switch t := token.(type) {
		case xml.StartElement:
			if cellDepth > 0 && drawingDepth > 0 {
				drawingDepth++
				captureDrawingImageRef(t, &drawing)
				continue
			}
			switch t.Name.Local {
			case "tr":
				if !inRow {
					inRow = true
					row = nil
				}
			case "tc":
				if inRow {
					if cellDepth == 0 {
						cell.Reset()
					}
					cellDepth++
				}
			case "drawing":
				if cellDepth > 0 {
					drawingDepth = 1
					drawing = drawingImageRef{}
				}
			case "t":
				if cellDepth > 0 && drawingDepth == 0 {
					textDepth++
				}
			}
		case xml.EndElement:
			if drawingDepth > 0 {
				drawingDepth--
				if drawingDepth == 0 {
					cell.WriteString(markdownImagePlaceholder(drawing))
				}
				continue
			}
			switch t.Name.Local {
			case "t":
				if textDepth > 0 {
					textDepth--
				}
			case "tc":
				if cellDepth > 0 {
					cellDepth--
					if cellDepth == 0 {
						row = append(row, normalizeMarkdownCell(cell.String()))
					}
				}
			case "tr":
				if inRow && cellDepth == 0 {
					rows = append(rows, row)
					inRow = false
					row = nil
				}
			}
		case xml.CharData:
			if textDepth > 0 && cellDepth > 0 {
				cell.Write([]byte(t))
			}
		}
	}

	return rows
}

func writeMarkdownRow(builder *strings.Builder, cells []string, columnCount int) {
	builder.WriteString("|")
	for i := 0; i < columnCount; i++ {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		builder.WriteString(" ")
		builder.WriteString(cell)
		builder.WriteString(" |")
	}
	builder.WriteString("\n")
}

func normalizeMarkdownCell(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.TrimSpace(text)
	return strings.ReplaceAll(text, "|", `\|`)
}
