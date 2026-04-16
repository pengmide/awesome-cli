package claude

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dream/internal/agent"
)

func TestClientRunStreamsRawStdoutAndExtractsFinalMessage(t *testing.T) {
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "fake-claude.sh")
	renderedStdout := "[claude] thinking...\n$ pwd\n" + tempDir + "/project\nTASK_DONE: first\nNEXT_TASK: second\n"
	script := `#!/bin/sh
set -eu
pwd >&2
session=""
settings=""
format=""
verbose=0
partials=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    -p)
      shift
      ;;
    --dangerously-skip-permissions)
      shift
      ;;
    --output-format)
      format="$2"
      shift 2
      ;;
    --verbose)
      verbose=1
      shift
      ;;
    --include-partial-messages)
      partials=1
      shift
      ;;
    --session-id)
      session="$2"
      shift 2
      ;;
    --settings)
      settings="$2"
      shift 2
      ;;
    *)
      prompt="$1"
      shift
      ;;
  esac
done
if [ "$session" != "123e4567-e89b-12d3-a456-426614174000" ]; then
  echo "unexpected session id: $session" >&2
  exit 2
fi
if [ "$settings" != "/tmp/claude-settings.json" ]; then
  echo "unexpected settings: $settings" >&2
  exit 3
fi
if [ "$format" != "stream-json" ]; then
  echo "unexpected output format: $format" >&2
  exit 5
fi
if [ "$verbose" -ne 1 ]; then
  echo "missing --verbose" >&2
  exit 7
fi
if [ "$partials" -ne 1 ]; then
  echo "missing --include-partial-messages" >&2
  exit 8
fi
if [ "$prompt" != "do work" ]; then
  echo "unexpected prompt: $prompt" >&2
  exit 6
fi
printf '%s\n' \
  '{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}}' \
  '{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"plan"}}}' \
  '{"type":"stream_event","event":{"type":"content_block_stop","index":0}}' \
  '{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","name":"Bash","input":{}}}}' \
  '{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"command\":\"pwd\"}"}}}' \
  '{"type":"stream_event","event":{"type":"content_block_stop","index":1}}' \
  '{"type":"user","tool_use_result":{"stdout":"` + tempDir + `/project\n"}}' \
  '{"type":"stream_event","event":{"type":"content_block_start","index":2,"content_block":{"type":"text","text":""}}}' \
  '{"type":"stream_event","event":{"type":"content_block_delta","index":2,"delta":{"type":"text_delta","text":"TASK_DONE: first\n"}}}' \
  '{"type":"stream_event","event":{"type":"content_block_delta","index":2,"delta":{"type":"text_delta","text":"NEXT_TASK: second\n"}}}' \
  '{"type":"stream_event","event":{"type":"content_block_stop","index":2}}' \
  '{"type":"result","result":"TASK_DONE: first\nNEXT_TASK: second\n","session_id":"123e4567-e89b-12d3-a456-426614174000"}'
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	stdoutLog := filepath.Join(tempDir, "stdout.log")
	stderrLog := filepath.Join(tempDir, "stderr.log")
	lastPath := filepath.Join(tempDir, "last.txt")
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	var stdout, stderr bytes.Buffer
	client := &Client{
		Binary: binary,
		Stdout: &stdout,
		Stderr: &stderr,
		SessionID: func() (string, error) {
			return "123e4567-e89b-12d3-a456-426614174000", nil
		},
	}

	result, err := client.Run(context.Background(), agent.RunRequest{
		ProjectRoot:     projectRoot,
		Prompt:          "do work",
		LastMessagePath: lastPath,
		StdoutLogPath:   stdoutLog,
		StderrLogPath:   stderrLog,
		Settings:        "/tmp/claude-settings.json",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.SessionID != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("unexpected session id: %s", result.SessionID)
	}
	if result.LastMessage != "TASK_DONE: first\nNEXT_TASK: second\n" {
		t.Fatalf("unexpected last message: %q", result.LastMessage)
	}
	if stdout.String() != renderedStdout {
		t.Fatalf("unexpected streamed stdout: %q", stdout.String())
	}

	lastData, err := os.ReadFile(lastPath)
	if err != nil {
		t.Fatalf("read last message: %v", err)
	}
	if string(lastData) != result.LastMessage {
		t.Fatalf("last message file mismatch: %q != %q", string(lastData), result.LastMessage)
	}

	stdoutData, err := os.ReadFile(stdoutLog)
	if err != nil {
		t.Fatalf("read stdout log: %v", err)
	}
	if string(stdoutData) != renderedStdout {
		t.Fatalf("stdout log mismatch: %q != %q", string(stdoutData), renderedStdout)
	}

	stderrData, err := os.ReadFile(stderrLog)
	if err != nil {
		t.Fatalf("read stderr log: %v", err)
	}
	if !strings.Contains(string(stderrData), projectRoot) {
		t.Fatalf("stderr log missing project root: %q", string(stderrData))
	}
}

func TestClientResumeUsesSavedSessionIDWithoutSettingsByDefault(t *testing.T) {
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "fake-claude.sh")
	script := `#!/bin/sh
set -eu
if [ "$1" != "-p" ]; then
  exit 1
fi
shift
if [ "$1" != "--dangerously-skip-permissions" ]; then
  echo "missing permissions flag" >&2
  exit 2
fi
shift
if [ "$1" != "--output-format" ]; then
  echo "missing output format flag" >&2
  exit 6
fi
shift
if [ "$1" != "stream-json" ]; then
  echo "wrong output format" >&2
  exit 7
fi
shift
if [ "$1" != "--verbose" ]; then
  echo "missing verbose flag" >&2
  exit 8
fi
shift
if [ "$1" != "--include-partial-messages" ]; then
  echo "missing partial flag" >&2
  exit 9
fi
shift
if [ "$1" != "--resume" ]; then
  echo "missing resume flag" >&2
  exit 3
fi
shift
if [ "$1" != "resume-session-1" ]; then
  echo "wrong session id order" >&2
  exit 4
fi
shift
if [ "$#" -ne 1 ]; then
  echo "unexpected extra args" >&2
  exit 5
fi
printf '%s\n' '{"type":"result","result":"ALL_TASKS_DONE\n"}'
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	client := &Client{Binary: binary}
	result, err := client.Resume(context.Background(), agent.ResumeRequest{
		SessionID:       "resume-session-1",
		Prompt:          "continue work",
		ProjectRoot:     projectRoot,
		LastMessagePath: filepath.Join(tempDir, "last.txt"),
		StdoutLogPath:   filepath.Join(tempDir, "stdout.log"),
		StderrLogPath:   filepath.Join(tempDir, "stderr.log"),
	})
	if err != nil {
		t.Fatalf("Resume returned error: %v", err)
	}
	if result.SessionID != "resume-session-1" {
		t.Fatalf("unexpected session id: %s", result.SessionID)
	}
	if !strings.Contains(result.LastMessage, "ALL_TASKS_DONE") {
		t.Fatalf("unexpected last message: %q", result.LastMessage)
	}
}

func TestExtractLastMessageReadsStreamJSONResult(t *testing.T) {
	lastMessage, sessionID, err := extractLastMessage(strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"session-1"}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ignore"}}}`,
		`{"type":"result","result":"TASK_DONE: first\nNEXT_TASK: second\n","session_id":"session-2"}`,
		"",
	}, "\n"))
	if err != nil {
		t.Fatalf("extractLastMessage returned error: %v", err)
	}
	if sessionID != "session-2" {
		t.Fatalf("unexpected session id: %q", sessionID)
	}
	if lastMessage != "TASK_DONE: first\nNEXT_TASK: second\n" {
		t.Fatalf("unexpected last message: %q", lastMessage)
	}
}

func TestClientRunRejectsEmptyFinalMessage(t *testing.T) {
	tempDir := t.TempDir()
	binary := filepath.Join(tempDir, "fake-claude.sh")
	script := `#!/bin/sh
set -eu
printf '\n'
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	client := &Client{
		Binary: binary,
		SessionID: func() (string, error) {
			return "123e4567-e89b-12d3-a456-426614174000", nil
		},
	}

	_, err := client.Run(context.Background(), agent.RunRequest{
		ProjectRoot:     tempDir,
		Prompt:          "do work",
		LastMessagePath: filepath.Join(tempDir, "last.txt"),
		StdoutLogPath:   filepath.Join(tempDir, "stdout.log"),
		StderrLogPath:   filepath.Join(tempDir, "stderr.log"),
	})
	if err == nil || !strings.Contains(err.Error(), "claude produced empty final message") {
		t.Fatalf("unexpected error: %v", err)
	}
}
