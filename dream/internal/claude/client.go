package claude

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"dream/internal/agent"
)

type Client struct {
	Binary    string
	Stdout    io.Writer
	Stderr    io.Writer
	SessionID func() (string, error)
}

func New(stdout, stderr io.Writer) *Client {
	return &Client{
		Binary:    "claude",
		Stdout:    stdout,
		Stderr:    stderr,
		SessionID: newSessionID,
	}
}

func (c *Client) Run(ctx context.Context, req agent.RunRequest) (agent.Result, error) {
	sessionIDFn := c.SessionID
	if sessionIDFn == nil {
		sessionIDFn = newSessionID
	}

	sessionID, err := sessionIDFn()
	if err != nil {
		return agent.Result{}, err
	}

	args := []string{
		"-p",
		"--dangerously-skip-permissions",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--session-id", sessionID,
	}
	if req.Settings != "" {
		args = append(args, "--settings", req.Settings)
	}
	args = append(args, req.Prompt)

	return c.execute(ctx, req.ProjectRoot, args, req.LastMessagePath, req.StdoutLogPath, req.StderrLogPath, sessionID)
}

func (c *Client) Resume(ctx context.Context, req agent.ResumeRequest) (agent.Result, error) {
	args := []string{
		"-p",
		"--dangerously-skip-permissions",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--resume", req.SessionID,
	}
	if req.Settings != "" {
		args = append(args, "--settings", req.Settings)
	}
	if req.Prompt != "" {
		args = append(args, req.Prompt)
	}

	return c.execute(ctx, req.ProjectRoot, args, req.LastMessagePath, req.StdoutLogPath, req.StderrLogPath, req.SessionID)
}

func (c *Client) execute(ctx context.Context, projectRoot string, args []string, lastMessagePath, stdoutLogPath, stderrLogPath, sessionID string) (agent.Result, error) {
	stdoutFile, err := os.OpenFile(stdoutLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return agent.Result{}, err
	}
	defer stdoutFile.Close()

	stderrFile, err := os.OpenFile(stderrLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return agent.Result{}, err
	}
	defer stderrFile.Close()

	command := exec.CommandContext(ctx, c.Binary, args...)
	command.Dir = projectRoot
	var stdoutBuffer bytes.Buffer
	streamWriter := newClaudeStreamWriter(&stdoutBuffer, newStreamRenderer(multiWriter(stdoutFile, c.Stdout)))
	command.Stdout = streamWriter
	command.Stderr = multiWriter(stderrFile, c.Stderr)

	runErr := command.Run()
	if err := streamWriter.Flush(); err != nil {
		return agent.Result{}, err
	}

	lastMessage, extractedSessionID, err := extractLastMessage(stdoutBuffer.String())
	if err != nil {
		return agent.Result{}, err
	}
	if extractedSessionID != "" {
		sessionID = extractedSessionID
	}
	if err := os.WriteFile(lastMessagePath, []byte(lastMessage), 0o644); err != nil {
		return agent.Result{}, err
	}
	if strings.TrimSpace(lastMessage) == "" && runErr == nil {
		return agent.Result{
			SessionID:   sessionID,
			LastMessage: lastMessage,
		}, fmt.Errorf("claude produced empty final message")
	}

	return agent.Result{
		SessionID:   sessionID,
		LastMessage: lastMessage,
	}, runErr
}

type printResponse struct {
	Type      string `json:"type"`
	Result    string `json:"result"`
	SessionID string `json:"session_id"`
}

func extractLastMessage(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw, "", nil
	}

	var lastMessage string
	var sessionID string
	foundResult := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var response printResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			continue
		}
		if response.Type != "result" {
			continue
		}
		lastMessage = response.Result
		if trimmedSessionID := strings.TrimSpace(response.SessionID); trimmedSessionID != "" {
			sessionID = trimmedSessionID
		}
		foundResult = true
	}
	if foundResult {
		return lastMessage, sessionID, nil
	}

	var response printResponse
	if err := json.Unmarshal([]byte(trimmed), &response); err == nil && response.Type == "result" {
		return response.Result, strings.TrimSpace(response.SessionID), nil
	}

	return raw, "", nil
}

type streamResponse struct {
	Type          string            `json:"type"`
	Event         *streamEvent      `json:"event,omitempty"`
	ToolUseResult *streamToolResult `json:"tool_use_result,omitempty"`
}

type streamEvent struct {
	Type         string              `json:"type"`
	Index        int                 `json:"index,omitempty"`
	ContentBlock *streamContentBlock `json:"content_block,omitempty"`
	Delta        *streamDelta        `json:"delta,omitempty"`
}

type streamContentBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name,omitempty"`
	Text  string          `json:"text,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type streamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

type streamToolResult struct {
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
}

type streamRenderer struct {
	out    io.Writer
	blocks map[int]*streamBlockState
}

type claudeStreamWriter struct {
	raw      *bytes.Buffer
	renderer *streamRenderer
	pending  bytes.Buffer
}

type streamBlockState struct {
	blockType         string
	toolName          string
	inputJSON         bytes.Buffer
	textNeedsNewline  bool
	thinkingAnnounced bool
}

func newStreamRenderer(out io.Writer) *streamRenderer {
	return &streamRenderer{
		out:    out,
		blocks: make(map[int]*streamBlockState),
	}
}

func newClaudeStreamWriter(raw *bytes.Buffer, renderer *streamRenderer) *claudeStreamWriter {
	return &claudeStreamWriter{
		raw:      raw,
		renderer: renderer,
	}
}

func (w *claudeStreamWriter) Write(data []byte) (int, error) {
	if _, err := w.raw.Write(data); err != nil {
		return 0, err
	}
	if _, err := w.pending.Write(data); err != nil {
		return 0, err
	}
	if err := w.consumeCompletedLines(); err != nil {
		return 0, err
	}
	return len(data), nil
}

func (w *claudeStreamWriter) Flush() error {
	if w.pending.Len() == 0 {
		return nil
	}
	line := strings.TrimSuffix(w.pending.String(), "\r")
	w.pending.Reset()
	if line == "" || w.renderer == nil {
		return nil
	}
	return w.renderer.ConsumeLine(line)
}

func (w *claudeStreamWriter) consumeCompletedLines() error {
	for {
		lineBytes := w.pending.Bytes()
		index := bytes.IndexByte(lineBytes, '\n')
		if index < 0 {
			return nil
		}
		line := strings.TrimSuffix(string(lineBytes[:index]), "\r")
		w.pending.Next(index + 1)
		if w.renderer != nil {
			if err := w.renderer.ConsumeLine(line); err != nil {
				return err
			}
		}
	}
}

func (r *streamRenderer) ConsumeLine(line string) error {
	var response streamResponse
	if err := json.Unmarshal([]byte(line), &response); err != nil {
		return nil
	}

	switch response.Type {
	case "stream_event":
		return r.consumeEvent(response.Event)
	case "user":
		return r.consumeToolResult(response.ToolUseResult)
	default:
		return nil
	}
}

func (r *streamRenderer) consumeEvent(event *streamEvent) error {
	if event == nil {
		return nil
	}

	switch event.Type {
	case "content_block_start":
		if event.ContentBlock == nil {
			return nil
		}
		state := &streamBlockState{
			blockType: event.ContentBlock.Type,
			toolName:  event.ContentBlock.Name,
		}
		if len(event.ContentBlock.Input) > 0 && string(event.ContentBlock.Input) != "null" && string(event.ContentBlock.Input) != "{}" {
			_, _ = state.inputJSON.Write(bytes.TrimSpace(event.ContentBlock.Input))
		}
		if event.ContentBlock.Type == "text" && event.ContentBlock.Text != "" {
			if err := r.writeText(state, event.ContentBlock.Text); err != nil {
				return err
			}
		}
		r.blocks[event.Index] = state
	case "content_block_delta":
		state := r.ensureBlock(event.Index)
		if event.Delta == nil {
			return nil
		}
		switch event.Delta.Type {
		case "text_delta":
			return r.writeText(state, event.Delta.Text)
		case "thinking_delta":
			if state.blockType == "thinking" && !state.thinkingAnnounced {
				state.thinkingAnnounced = true
				return writeWithTrailingNewline(r.out, "[claude] thinking...")
			}
		case "input_json_delta":
			_, _ = state.inputJSON.WriteString(event.Delta.PartialJSON)
		}
	case "content_block_stop":
		state := r.blocks[event.Index]
		if state == nil {
			return nil
		}
		defer delete(r.blocks, event.Index)
		switch state.blockType {
		case "text":
			if state.textNeedsNewline {
				_, err := io.WriteString(r.out, "\n")
				return err
			}
		case "tool_use":
			toolLine := formatToolUse(state.toolName, strings.TrimSpace(state.inputJSON.String()))
			if toolLine != "" {
				return writeWithTrailingNewline(r.out, toolLine)
			}
		}
	}

	return nil
}

func (r *streamRenderer) consumeToolResult(result *streamToolResult) error {
	if result == nil {
		return nil
	}
	if strings.TrimSpace(result.Stdout) != "" {
		if err := writeWithTrailingNewline(r.out, result.Stdout); err != nil {
			return err
		}
	}
	if strings.TrimSpace(result.Stderr) != "" {
		if err := writeWithTrailingNewline(r.out, result.Stderr); err != nil {
			return err
		}
	}
	return nil
}

func (r *streamRenderer) ensureBlock(index int) *streamBlockState {
	state := r.blocks[index]
	if state == nil {
		state = &streamBlockState{}
		r.blocks[index] = state
	}
	return state
}

func (r *streamRenderer) writeText(state *streamBlockState, text string) error {
	if text == "" {
		return nil
	}
	if _, err := io.WriteString(r.out, text); err != nil {
		return err
	}
	state.textNeedsNewline = !strings.HasSuffix(text, "\n")
	return nil
}

func formatToolUse(name, rawInput string) string {
	if name == "" {
		return ""
	}

	if name == "Bash" {
		var payload struct {
			Command     string `json:"command"`
			Description string `json:"description"`
		}
		if rawInput != "" && json.Unmarshal([]byte(rawInput), &payload) == nil {
			if command := strings.TrimSpace(payload.Command); command != "" {
				return "$ " + command
			}
			if description := strings.TrimSpace(payload.Description); description != "" {
				return "[Bash] " + description
			}
		}
	}

	if rawInput == "" || rawInput == "{}" {
		return fmt.Sprintf("[%s]", name)
	}
	return fmt.Sprintf("[%s] %s", name, rawInput)
}

func writeWithTrailingNewline(writer io.Writer, text string) error {
	if _, err := io.WriteString(writer, text); err != nil {
		return err
	}
	if strings.HasSuffix(text, "\n") {
		return nil
	}
	_, err := io.WriteString(writer, "\n")
	return err
}

func multiWriter(writers ...io.Writer) io.Writer {
	filtered := make([]io.Writer, 0, len(writers))
	for _, writer := range writers {
		if writer != nil {
			filtered = append(filtered, writer)
		}
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return io.MultiWriter(filtered...)
}

func newSessionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}

	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		raw[0:4],
		raw[4:6],
		raw[6:8],
		raw[8:10],
		raw[10:16],
	), nil
}
