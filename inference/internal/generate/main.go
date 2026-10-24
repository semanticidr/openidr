//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	inputFile          = "diagnosis_result.json"
	diagnosisSchemaDir = "internal/diagnosisschema"
	diagnosisOutput    = "internal/diagnosisschema/generated_diagnosis.go"
	resultOutput       = "generated_result.go"
)

func main() {
	// 1. Early Failing Validations
	quicktypePath, err := checkDependencies()
	if err != nil {
		fatal(err)
	}

	schemaData, err := extractSchema(inputFile)
	if err != nil {
		fatal(err)
	}

	// 2. Code Generation
	if err := generateTypes(quicktypePath, schemaData); err != nil {
		fatal(err)
	}

	// 3. Post-Processing
	if err := formatFiles([]string{diagnosisOutput, resultOutput}); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func checkDependencies() (string, error) {
	quicktypePath, err := exec.LookPath("quicktype")
	if err != nil {
		return "", fmt.Errorf("warning: quicktype is required to generate diagnosis types: %w; install Node.js and npm, then run `npm install -g quicktype` and ensure quicktype is on PATH", err)
	}

	if _, err := exec.LookPath("gofmt"); err != nil {
		return "", fmt.Errorf("warning: gofmt is required but not found on PATH: %w", err)
	}

	return quicktypePath, nil
}

func extractSchema(filePath string) ([]byte, error) {
	document, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read input file: %w", err)
	}

	var wrapper struct {
		Schema json.RawMessage `json:"schema"`
	}
	if err := json.Unmarshal(document, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal input JSON: %w", err)
	}

	if len(wrapper.Schema) == 0 || string(wrapper.Schema) == "null" {
		return nil, fmt.Errorf("diagnosis document is missing its schema")
	}

	return wrapper.Schema, nil
}

func generateTypes(quicktypePath string, schemaData []byte) error {
	dir, err := os.MkdirTemp("", "diagnosis-generation-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	schemaPath := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schemaPath, schemaData, 0600); err != nil {
		return fmt.Errorf("write temp schema file: %w", err)
	}

	if err := os.MkdirAll(diagnosisSchemaDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	typeConfigs := [][]string{
		{inputFile, "--src-lang", "json", "--top-level", "GeneratedDiagnosis", "--out", diagnosisOutput, "--package", "diagnosisschema"},
		{schemaPath, "--src-lang", "schema", "--top-level", "DiagnosisResult", "--out", resultOutput, "--no-date-times", "--package", "inference"},
	}

	for _, args := range typeConfigs {
		args = append(args, "--lang", "go")
		cmd := exec.Command(quicktypePath, args...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("run quicktype for %s: %w", args[6], err)
		}
	}

	return nil
}

func formatFiles(paths []string) error {
	for _, path := range paths {
		if output, err := exec.Command("gofmt", "-w", path).CombinedOutput(); err != nil {
			return fmt.Errorf("format %s: %w: %s", path, err, output)
		}
	}
	return nil
}
