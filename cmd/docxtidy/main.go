package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kiry163/docxtidy"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	switch args[0] {
	case "extract":
		return runExtract(args[1:])
	case "outline":
		return runOutline(args[1:])
	case "apply":
		return runApply(args[1:])
	case "write":
		return runWrite(args[1:])
	case "-h", "--help", "help":
		printUsage()
		return nil
	default:
		return usageError()
	}
}

func runExtract(args []string) error {
	inputPath, outDir, err := parseInputAndOut(args)
	if err != nil {
		return err
	}
	if inputPath == "" || outDir == "" {
		return fmt.Errorf("usage: docxtidy extract <input.docx> --out <snapshot.json>")
	}

	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input docx: %w", err)
	}
	defer input.Close()

	snapshot, err := docxtidy.Extract(context.Background(), input, docxtidy.ExtractOptions{})
	if err != nil {
		return err
	}
	if err := writeJSON(outDir, snapshot); err != nil {
		return err
	}
	fmt.Println(outDir)
	return nil
}

func runOutline(args []string) error {
	snapshotPath, outPath, err := parseInputAndOut(args)
	if err != nil {
		return err
	}
	if snapshotPath == "" || outPath == "" {
		return fmt.Errorf("usage: docxtidy outline <snapshot.json> --out <outline.json>")
	}

	var snapshot docxtidy.Snapshot
	if err := readJSON(snapshotPath, &snapshot); err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	outline := docxtidy.OutlineOf(snapshot, docxtidy.OutlineOptions{})
	if err := writeJSON(outPath, outline); err != nil {
		return err
	}
	fmt.Println(outPath)
	return nil
}

func runApply(args []string) error {
	snapshotPath, options, err := parseInputAndOptions(args)
	if err != nil {
		return err
	}
	layoutPath := options["layout"]
	outPath := options["out"]
	if snapshotPath == "" || layoutPath == "" || outPath == "" {
		return fmt.Errorf("usage: docxtidy apply <snapshot.json> --layout <layout.json> --out <updated-snapshot.json>")
	}

	var snapshot docxtidy.Snapshot
	if err := readJSON(snapshotPath, &snapshot); err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}
	var layout docxtidy.Layout
	if err := readJSON(layoutPath, &layout); err != nil {
		return fmt.Errorf("read layout: %w", err)
	}

	updated, err := docxtidy.Apply(context.Background(), snapshot, layout)
	if err != nil {
		return err
	}
	if err := writeJSON(outPath, updated); err != nil {
		return err
	}
	fmt.Println(outPath)
	return nil
}

func runWrite(args []string) error {
	snapshotPath, outPath, err := parseInputAndOut(args)
	if err != nil {
		return err
	}
	if snapshotPath == "" || outPath == "" {
		return fmt.Errorf("usage: docxtidy write <snapshot.json> --out <output.docx>")
	}

	var snapshot docxtidy.Snapshot
	if err := readJSON(snapshotPath, &snapshot); err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	output, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output docx: %w", err)
	}
	defer output.Close()

	if err := docxtidy.Write(context.Background(), snapshot, output); err != nil {
		return err
	}
	fmt.Println(outPath)
	return nil
}

func usageError() error {
	printUsage()
	return fmt.Errorf("invalid command")
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  docxtidy extract <input.docx> --out <snapshot.json>")
	fmt.Fprintln(os.Stderr, "  docxtidy outline <snapshot.json> --out <outline.json>")
	fmt.Fprintln(os.Stderr, "  docxtidy apply <snapshot.json> --layout <layout.json> --out <updated-snapshot.json>")
	fmt.Fprintln(os.Stderr, "  docxtidy write <snapshot.json> --out <output.docx>")
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	return nil
}

func writeJSON(path string, value any) error {
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

func parseInputAndOut(args []string) (string, string, error) {
	var input string
	var out string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--out", "-out":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s requires a value", arg)
			}
			out = args[i+1]
			i++
		default:
			if input != "" {
				return "", "", fmt.Errorf("unexpected argument: %s", arg)
			}
			input = arg
		}
	}

	return input, out, nil
}

func parseInputAndOptions(args []string) (string, map[string]string, error) {
	var input string
	options := map[string]string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			key := strings.TrimPrefix(arg, "--")
			if key == "" {
				return "", nil, fmt.Errorf("empty option name")
			}
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s requires a value", arg)
			}
			options[key] = args[i+1]
			i++
			continue
		}

		if input != "" {
			return "", nil, fmt.Errorf("unexpected argument: %s", arg)
		}
		input = arg
	}

	return input, options, nil
}
