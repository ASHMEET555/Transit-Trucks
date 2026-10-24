package app

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findRepoRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "TRANSITOPS_GO_PRODUCT_PLAN.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root")
		}
		dir = parent
	}
}

func TestHygiene_NoPassword123InSource(t *testing.T) {
	root := findRepoRoot(t)

	// Scan backend-go and frontend/src (excluding test files, documentation, and vendor/node_modules)
	targets := []string{
		filepath.Join(root, "backend-go", "cmd"),
		filepath.Join(root, "backend-go", "internal"),
		filepath.Join(root, "frontend", "src"),
	}

	for _, target := range targets {
		err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			// Skip test files
			if strings.HasSuffix(path, "_test.go") || strings.Contains(path, "__tests__") || strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".test.tsx") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(content), "password123") {
				t.Errorf("forbidden literal 'password123' found in %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk failed: %v", err)
		}
	}
}

func TestHygiene_NoLocalStorageInFrontendAuth(t *testing.T) {
	root := findRepoRoot(t)
	frontendSrc := filepath.Join(root, "frontend", "src")

	err := filepath.WalkDir(frontendSrc, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// Only check ts/tsx files
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".tsx") {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(line, "localStorage") || strings.Contains(line, "sessionStorage") {
				t.Errorf("forbidden web storage call found in %s:%d: %s", path, lineNum, line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk failed on frontend/src: %v", err)
	}
}
