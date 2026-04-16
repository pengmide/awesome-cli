package doc

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const frontScanLines = 80

func EnsureProjectRoot(docPath, workingDir string, logOut io.Writer) (string, bool, error) {
	contentBytes, err := os.ReadFile(docPath)
	if err != nil {
		return "", false, err
	}
	content := string(contentBytes)

	if value, _, found := findFrontProjectRoot(content); found {
		projectRoot, err := validateProjectRoot(value)
		if err == nil {
			return projectRoot, false, nil
		}

		projectRoot, err = validateProjectRoot(workingDir)
		if err != nil {
			return "", false, err
		}
		if logOut != nil {
			_, _ = fmt.Fprintf(logOut, "PROJECT_ROOT in %s is invalid; using current working directory: %s\n", docPath, projectRoot)
		}
		return writeProjectRoot(docPath, content, projectRoot)
	}

	projectRoot, err := validateProjectRoot(workingDir)
	if err != nil {
		return "", false, err
	}
	if logOut != nil {
		_, _ = fmt.Fprintf(logOut, "PROJECT_ROOT not found in %s; using current working directory: %s\n", docPath, projectRoot)
	}
	return writeProjectRoot(docPath, content, projectRoot)
}

func writeProjectRoot(docPath, content, projectRoot string) (string, bool, error) {
	updated := upsertProjectRoot(content, projectRoot)
	info, err := os.Stat(docPath)
	if err != nil {
		return "", false, err
	}
	if err := os.WriteFile(docPath, []byte(updated), info.Mode()); err != nil {
		return "", false, err
	}
	return projectRoot, true, nil
}

func validateProjectRoot(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("path is empty")
	}
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("path must be absolute")
	}

	clean := filepath.Clean(value)
	info, err := os.Stat(clean)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", clean)
	}
	return clean, nil
}

func findFrontProjectRoot(content string) (string, int, bool) {
	lines := splitLines(content)
	limit := frontScanLines
	if len(lines) < limit {
		limit = len(lines)
	}

	for i := 0; i < limit; i++ {
		line := strings.TrimSpace(strings.TrimRight(lines[i], "\r\n"))
		if !strings.HasPrefix(line, "PROJECT_ROOT=") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "PROJECT_ROOT=")), i, true
	}
	return "", -1, false
}

func upsertProjectRoot(content, projectRoot string) string {
	lines := splitLines(content)
	newline := detectNewline(content)

	if _, index, found := findFrontProjectRoot(content); found {
		suffix := lineSuffix(lines[index])
		if suffix == "" {
			suffix = newline
		}
		lines[index] = "PROJECT_ROOT=" + projectRoot + suffix
		return strings.Join(lines, "")
	}

	header := "PROJECT_ROOT=" + projectRoot + newline
	if content == "" {
		return header
	}
	return header + newline + content
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}

	lines := make([]string, 0, strings.Count(content, "\n")+1)
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, content[start:i+1])
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, content[start:])
	}
	return lines
}

func detectNewline(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func lineSuffix(line string) string {
	switch {
	case strings.HasSuffix(line, "\r\n"):
		return "\r\n"
	case strings.HasSuffix(line, "\n"):
		return "\n"
	default:
		return ""
	}
}
