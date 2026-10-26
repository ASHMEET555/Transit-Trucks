package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSeedProductionGuard(t *testing.T) {
	cmd := exec.Command("go", "run", "main.go")
	cmd.Env = append(os.Environ(), "ENV=production", "DATABASE_URL=postgresql://invalid:invalid@localhost:5432/invalid")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected cmd/seed with ENV=production to exit non-zero, but succeeded. Output: %s", string(out))
	}
	outStr := string(out)
	if !strings.Contains(outStr, "ENV=production") {
		t.Fatalf("expected output to mention ENV=production, got: %s", outStr)
	}
}
