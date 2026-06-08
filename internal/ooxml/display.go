package ooxml

import (
	"encoding/xml"
	"io"
	"strings"
)

type drawingImageRef struct {
	name string
	rel  string
}

func DisplayTextFromBlockXML(blockXML string, fallback string) string {
	decoder := xml.NewDecoder(strings.NewReader(blockXML))
	var builder strings.Builder
	textDepth := 0
	drawingDepth := 0
	drawing := drawingImageRef{}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fallback
		}

		switch t := token.(type) {
		case xml.StartElement:
			if drawingDepth > 0 {
				drawingDepth++
				captureDrawingImageRef(t, &drawing)
				continue
			}
			switch t.Name.Local {
			case "drawing":
				drawingDepth = 1
				drawing = drawingImageRef{}
			case "t":
				textDepth++
			}
		case xml.EndElement:
			if drawingDepth > 0 {
				drawingDepth--
				if drawingDepth == 0 {
					builder.WriteString(markdownImagePlaceholder(drawing))
				}
				continue
			}
			if t.Name.Local == "t" && textDepth > 0 {
				textDepth--
			}
		case xml.CharData:
			if textDepth > 0 {
				builder.Write([]byte(t))
			}
		}
	}

	text := builder.String()
	if text == "" {
		return fallback
	}
	return text
}

func captureDrawingImageRef(element xml.StartElement, ref *drawingImageRef) {
	switch element.Name.Local {
	case "docPr", "cNvPr":
		if ref.name == "" {
			ref.name = attrValue(element, "name")
		}
		if ref.name == "" {
			ref.name = attrValue(element, "descr")
		}
	case "blip":
		if ref.rel == "" {
			ref.rel = attrValue(element, "embed")
		}
		if ref.rel == "" {
			ref.rel = attrValue(element, "link")
		}
	}
}

func markdownImagePlaceholder(ref drawingImageRef) string {
	name := ref.name
	if name == "" {
		name = "image"
	}
	rel := ref.rel
	if rel == "" {
		rel = "unknown"
	}
	return "![" + escapeMarkdownImageAlt(name) + "](media:" + escapeMarkdownImageTarget(rel) + ")"
}

func escapeMarkdownImageAlt(text string) string {
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = strings.ReplaceAll(text, `]`, `\]`)
	return strings.ReplaceAll(text, "\n", " ")
}

func escapeMarkdownImageTarget(text string) string {
	text = strings.ReplaceAll(text, ")", "%29")
	return strings.ReplaceAll(text, " ", "%20")
}
