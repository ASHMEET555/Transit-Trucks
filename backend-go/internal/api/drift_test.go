package api

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAPITypesDrift(t *testing.T) {
	// Find repo root and openapi.yaml
	yamlPath := filepath.Join("..", "..", "..", "api", "openapi.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		yamlPath = filepath.Join("..", "..", "api", "openapi.yaml")
	}
	if _, err := os.Stat(yamlPath); err != nil {
		t.Fatalf("cannot locate openapi.yaml at %s", yamlPath)
	}

	currentFile := "types.gen.go"
	currentBytes, err := os.ReadFile(currentFile)
	if err != nil {
		t.Fatalf("cannot read %s: %v", currentFile, err)
	}

	tempFile := filepath.Join(t.TempDir(), "types.gen.go")

	cmd := exec.Command("go", "run", "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1",
		"-generate", "types", "-package", "api", "-o", tempFile, yamlPath)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oapi-codegen execution failed: %v, output: %s", err, string(out))
	}

	generatedBytes, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("cannot read generated temp file: %v", err)
	}

	// Normalize CRLF vs LF
	normalize := func(s string) string {
		return strings.ReplaceAll(s, "\r\n", "\n")
	}

	if normalize(string(currentBytes)) != normalize(string(generatedBytes)) {
		t.Fatalf("OpenAPI Go types are out of date with api/openapi.yaml! Regenerate using oapi-codegen.")
	}
}
