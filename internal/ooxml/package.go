package ooxml

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
)

const documentXMLPath = "word/document.xml"

func Extract(ctx context.Context, r io.Reader) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return State{}, fmt.Errorf("read docx: %w", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return State{}, fmt.Errorf("open docx: %w", err)
	}

	var files []PackageFile
	var documentXML []byte
	var numberingXML []byte
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return State{}, err
		}
		if file.FileInfo().IsDir() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return State{}, fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}
		fileData, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return State{}, fmt.Errorf("read zip entry %s: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return State{}, fmt.Errorf("close zip entry %s: %w", file.Name, closeErr)
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
		return State{}, fmt.Errorf("docx missing %s", documentXMLPath)
	}

	numberingDefinitions, err := parseNumberingDefinitions(numberingXML)
	if err != nil {
		return State{}, err
	}
	blocks, err := bodyBlocks(documentXML, numberingDefinitions)
	if err != nil {
		return State{}, err
	}

	return State{
		Blocks: blocks,
		Files:  files,
	}, nil
}

func Write(ctx context.Context, state State, w io.Writer) error {
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
			rebuilt, err := replaceBodyBlocks(data, state.Blocks)
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
