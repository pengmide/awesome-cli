package codex

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"dream/internal/agent"
)

var sessionIDPattern = regexp.MustCompile(`(?mi)^session id:\s*(.+?)\s*$`)

type Client struct {
	Binary string
	Stdout io.Writer
	Stderr io.Writer
}

func New(stdout, stderr io.Writer) *Client {
	return &Client{
		Binary: "codex",
		Stdout: stdout,
		Stderr: stderr,
	}
}

func (c *Client) Run(ctx context.Context, req agent.RunRequest) (agent.Result, error) {
	args := []string{
		"exec",
		"--skip-git-repo-check",
		"-C", req.ProjectRoot,
		"-s", "danger-full-access",
		"--color", "never",
		"-o", req.LastMessagePath,
		"-",
	}
	return c.execute(ctx, args, strings.NewReader(req.Prompt), req.StdoutLogPath, req.StderrLogPath, "")
}

func (c *Client) Resume(ctx context.Context, req agent.ResumeRequest) (agent.Result, error) {
	args := []string{
		"exec",
		"resume",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-o", req.LastMessagePath,
		req.SessionID,
	}
	if req.Prompt != "" {
		args = append(args, req.Prompt)
	}
	return c.execute(ctx, args, nil, req.StdoutLogPath, req.StderrLogPath, req.SessionID)
}

func (c *Client) execute(ctx context.Context, args []string, stdin io.Reader, stdoutLogPath, stderrLogPath, fallbackSessionID string) (agent.Result, error) {
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
	command.Stdin = stdin
	command.Stdout = multiWriter(stdoutFile, c.Stdout)

	var stderrBuffer bytes.Buffer
	command.Stderr = multiWriter(stderrFile, &stderrBuffer, c.Stderr)

	runErr := command.Run()

	lastMessagePath, err := lastMessagePathFromArgs(args)
	if err != nil {
		return agent.Result{}, err
	}

	lastMessage, err := readLastMessage(lastMessagePath)
	if err != nil {
		return agent.Result{}, err
	}

	sessionID := fallbackSessionID
	if extracted := ExtractSessionID(stderrBuffer.String()); extracted != "" {
		sessionID = extracted
	}

	return agent.Result{
		SessionID:   sessionID,
		LastMessage: lastMessage,
	}, runErr
}

func ExtractSessionID(stderr string) string {
	matches := sessionIDPattern.FindStringSubmatch(stderr)
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
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

func lastMessagePathFromArgs(args []string) (string, error) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-o" {
			return args[i+1], nil
		}
	}
	return "", fmt.Errorf("missing -o argument")
}

func readLastMessage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}
