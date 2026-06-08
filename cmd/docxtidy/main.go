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
	case "view":
		return runView(args[1:])
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
		return fmt.Errorf("usage: docxtidy extract <input.docx> --out <state.json>")
	}

	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input docx: %w", err)
	}
	defer input.Close()

	state, err := docxtidy.Extract(context.Background(), input, docxtidy.ExtractOptions{})
	if err != nil {
		return err
	}
	if err := writeJSON(outDir, state); err != nil {
		return err
	}
	fmt.Println(outDir)
	return nil
}

func runView(args []string) error {
	statePath, outPath, err := parseInputAndOut(args)
	if err != nil {
		return err
	}
	if statePath == "" || outPath == "" {
		return fmt.Errorf("usage: docxtidy view <state.json> --out <view.json>")
	}

	var state docxtidy.State
	if err := readJSON(statePath, &state); err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	view := docxtidy.ViewOf(state, docxtidy.ViewOptions{})
	if err := writeJSON(outPath, view); err != nil {
		return err
	}
	fmt.Println(outPath)
	return nil
}

func runApply(args []string) error {
	statePath, options, err := parseInputAndOptions(args)
	if err != nil {
		return err
	}
	structurePath := options["structure"]
	transformPath := options["transform"]
	outPath := options["out"]
	if statePath == "" || structurePath == "" || transformPath == "" || outPath == "" {
		return fmt.Errorf("usage: docxtidy apply <state.json> --structure <structure.json> --transform <transform.json> --out <new-state.json>")
	}

	var state docxtidy.State
	if err := readJSON(statePath, &state); err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	var structure docxtidy.Structure
	if err := readJSON(structurePath, &structure); err != nil {
		return fmt.Errorf("read structure: %w", err)
	}
	var transform docxtidy.Transform
	if err := readJSON(transformPath, &transform); err != nil {
		return fmt.Errorf("read transform: %w", err)
	}

	updated, err := docxtidy.Apply(context.Background(), state, structure, transform)
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
	statePath, outPath, err := parseInputAndOut(args)
	if err != nil {
		return err
	}
	if statePath == "" || outPath == "" {
		return fmt.Errorf("usage: docxtidy write <state.json> --out <output.docx>")
	}

	var state docxtidy.State
	if err := readJSON(statePath, &state); err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	output, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output docx: %w", err)
	}
	defer output.Close()

	if err := docxtidy.Write(context.Background(), state, output); err != nil {
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
	fmt.Fprintln(os.Stderr, "  docxtidy extract <input.docx> --out <state.json>")
	fmt.Fprintln(os.Stderr, "  docxtidy view <state.json> --out <view.json>")
	fmt.Fprintln(os.Stderr, "  docxtidy apply <state.json> --structure <structure.json> --transform <transform.json> --out <new-state.json>")
	fmt.Fprintln(os.Stderr, "  docxtidy write <state.json> --out <output.docx>")
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
