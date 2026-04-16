package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dream/internal/doc"
)

func TestCreateGeneratesTemplateFiles(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "my-plan")

	result, err := Create(target)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if result.Dir != target {
		t.Fatalf("unexpected result dir: %s", result.Dir)
	}
	if result.ReadmePath != filepath.Join(target, ReadmeFilename) {
		t.Fatalf("unexpected README path: %s", result.ReadmePath)
	}
	if result.FeaturePath != filepath.Join(target, FeatureFilename) {
		t.Fatalf("unexpected feature path: %s", result.FeaturePath)
	}

	readme, err := os.ReadFile(result.ReadmePath)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	readmeText := string(readme)
	if !strings.Contains(readmeText, "## 文档总表") {
		t.Fatalf("README missing document table:\n%s", readmeText)
	}
	if !strings.Contains(readmeText, "[feature-foundation.md](./feature-foundation.md)") {
		t.Fatalf("README missing feature link:\n%s", readmeText)
	}
	if strings.HasPrefix(readmeText, "PROJECT_ROOT=") {
		t.Fatalf("README should not start with PROJECT_ROOT before first run:\n%s", readmeText)
	}

	feature, err := os.ReadFile(result.FeaturePath)
	if err != nil {
		t.Fatalf("read feature doc: %v", err)
	}
	featureText := string(feature)
	if !strings.Contains(featureText, "## 细任务拆分") {
		t.Fatalf("feature doc missing task table:\n%s", featureText)
	}
	if !strings.Contains(featureText, "| F1 | 待补充首个子任务 |") {
		t.Fatalf("feature doc missing default task rows:\n%s", featureText)
	}
	if !strings.Contains(featureText, "### 记录模板") {
		t.Fatalf("feature doc missing completion template:\n%s", featureText)
	}
}

func TestCreateRejectsNonEmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "my-plan")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "existing.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	_, err := Create(target)
	if err == nil || !strings.Contains(err.Error(), "is not empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRejectsExistingFilePath(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "not-a-dir")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	_, err := Create(target)
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeneratedReadmeCanBeAssignedProjectRoot(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	result, err := Create(filepath.Join(tempDir, "my-plan"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	root, updated, err := doc.EnsureProjectRoot(result.ReadmePath, projectRoot, nil)
	if err != nil {
		t.Fatalf("EnsureProjectRoot returned error: %v", err)
	}
	if !updated {
		t.Fatalf("expected README to be updated with PROJECT_ROOT")
	}
	if root != projectRoot {
		t.Fatalf("unexpected project root: %s", root)
	}

	content, err := os.ReadFile(result.ReadmePath)
	if err != nil {
		t.Fatalf("read README after update: %v", err)
	}
	if !strings.HasPrefix(string(content), "PROJECT_ROOT="+projectRoot+"\n\n# 工作计划\n") {
		t.Fatalf("README not updated as expected:\n%s", string(content))
	}
}
