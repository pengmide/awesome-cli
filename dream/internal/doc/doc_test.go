package doc

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureProjectRootUsesWorkingDirWhenMissing(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(docPath, []byte("# Plan\n\nDo something.\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	var log bytes.Buffer
	root, changed, err := EnsureProjectRoot(docPath, projectRoot, &log)
	if err != nil {
		t.Fatalf("EnsureProjectRoot returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected document to change")
	}
	if root != projectRoot {
		t.Fatalf("unexpected project root: %s", root)
	}

	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	if got := string(content); !strings.HasPrefix(got, "PROJECT_ROOT="+projectRoot+"\n\n") {
		t.Fatalf("document head not updated correctly:\n%s", got)
	}
	if !strings.Contains(log.String(), "using current working directory") {
		t.Fatalf("expected current working directory log, got %q", log.String())
	}
}

func TestEnsureProjectRootReplacesInvalidValueWithWorkingDir(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(docPath, []byte("PROJECT_ROOT=relative/path\n\n# Plan\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	var log bytes.Buffer
	root, changed, err := EnsureProjectRoot(docPath, projectRoot, &log)
	if err != nil {
		t.Fatalf("EnsureProjectRoot returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected document to change")
	}
	if root != projectRoot {
		t.Fatalf("unexpected project root: %s", root)
	}

	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	if got := string(content); !strings.HasPrefix(got, "PROJECT_ROOT="+projectRoot+"\n\n") {
		t.Fatalf("document head not updated correctly:\n%s", got)
	}
	if !strings.Contains(log.String(), "is invalid") {
		t.Fatalf("expected invalid project root log, got %q", log.String())
	}
}

func TestEnsureProjectRootUsesExistingValidValue(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	original := "PROJECT_ROOT=" + projectRoot + "\n\n# Plan\n"
	if err := os.WriteFile(docPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	root, changed, err := EnsureProjectRoot(docPath, tempDir, nil)
	if err != nil {
		t.Fatalf("EnsureProjectRoot returned error: %v", err)
	}
	if changed {
		t.Fatalf("expected no document change")
	}
	if root != projectRoot {
		t.Fatalf("unexpected project root: %s", root)
	}

	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	if string(content) != original {
		t.Fatalf("document changed unexpectedly:\n%s", string(content))
	}
}
