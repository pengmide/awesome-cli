package codex

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dream/internal/agent"
)

func TestClientRunCapturesLogsAndSessionID(t *testing.T) {
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "fake-codex.sh")
	script := `#!/bin/sh
set -eu
cmd="$1"
shift
if [ "$cmd" != "exec" ]; then
  exit 1
fi
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --skip-git-repo-check)
      shift
      ;;
    -C)
      shift 2
      ;;
    -s)
      shift 2
      ;;
    --color)
      shift 2
      ;;
    -o)
      out="$2"
      shift 2
      ;;
    -)
      cat >/dev/null
      shift
      ;;
    *)
      shift
      ;;
  esac
done
echo "hello from codex"
echo "session id: fake-session-1" >&2
printf 'TASK_DONE: first task\nNEXT_TASK: second task\n' > "$out"
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	stdoutLog := filepath.Join(tempDir, "stdout.log")
	stderrLog := filepath.Join(tempDir, "stderr.log")
	lastPath := filepath.Join(tempDir, "last.txt")

	var stdout, stderr bytes.Buffer
	client := &Client{
		Binary: binary,
		Stdout: &stdout,
		Stderr: &stderr,
	}

	result, err := client.Run(context.Background(), agent.RunRequest{
		ProjectRoot:     tempDir,
		Prompt:          "do work",
		LastMessagePath: lastPath,
		StdoutLogPath:   stdoutLog,
		StderrLogPath:   stderrLog,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.SessionID != "fake-session-1" {
		t.Fatalf("unexpected session id: %s", result.SessionID)
	}
	if !strings.Contains(result.LastMessage, "TASK_DONE: first task") {
		t.Fatalf("unexpected last message: %q", result.LastMessage)
	}

	stdoutData, err := os.ReadFile(stdoutLog)
	if err != nil {
		t.Fatalf("read stdout log: %v", err)
	}
	if !strings.Contains(string(stdoutData), "hello from codex") {
		t.Fatalf("stdout log missing content: %q", string(stdoutData))
	}

	stderrData, err := os.ReadFile(stderrLog)
	if err != nil {
		t.Fatalf("read stderr log: %v", err)
	}
	if !strings.Contains(string(stderrData), "session id: fake-session-1") {
		t.Fatalf("stderr log missing session id: %q", string(stderrData))
	}
}

func TestClientResumePlacesOptionsBeforeSessionID(t *testing.T) {
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "fake-codex.sh")
	script := `#!/bin/sh
set -eu
if [ "$1" != "exec" ] || [ "$2" != "resume" ]; then
  exit 1
fi
shift 2
out=""
last=""
	while [ "$#" -gt 0 ]; do
  case "$1" in
    --skip-git-repo-check)
      shift
      ;;
    --dangerously-bypass-approvals-and-sandbox)
      shift
      ;;
    --color)
      shift 2
      ;;
    -o)
      out="$2"
      shift 2
      ;;
    *)
      last="$1"
      shift
      ;;
  esac
done
if [ "$last" != "resume-session-1" ]; then
  echo "session id position is wrong" >&2
  exit 3
fi
printf 'ALL_TASKS_DONE\n' > "$out"
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	client := &Client{Binary: binary}
	_, err := client.Resume(context.Background(), agent.ResumeRequest{
		SessionID:       "resume-session-1",
		LastMessagePath: filepath.Join(tempDir, "last.txt"),
		StdoutLogPath:   filepath.Join(tempDir, "stdout.log"),
		StderrLogPath:   filepath.Join(tempDir, "stderr.log"),
	})
	if err != nil {
		t.Fatalf("Resume returned error: %v", err)
	}
}
