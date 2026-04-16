package agent

import (
	"context"
	"strings"
)

const (
	ModelCodex  = "codex"
	ModelClaude = "claude"
)

type Executor interface {
	Run(context.Context, RunRequest) (Result, error)
	Resume(context.Context, ResumeRequest) (Result, error)
}

type RunRequest struct {
	ProjectRoot     string
	Prompt          string
	LastMessagePath string
	StdoutLogPath   string
	StderrLogPath   string
	Settings        string
}

type ResumeRequest struct {
	SessionID       string
	Prompt          string
	ProjectRoot     string
	LastMessagePath string
	StdoutLogPath   string
	StderrLogPath   string
	Settings        string
}

type Result struct {
	SessionID   string
	LastMessage string
}

func NormalizeModel(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "", ModelCodex:
		return ModelCodex
	case ModelClaude:
		return ModelClaude
	default:
		return strings.ToLower(strings.TrimSpace(model))
	}
}

func IsSupportedModel(model string) bool {
	switch NormalizeModel(model) {
	case ModelCodex, ModelClaude:
		return true
	default:
		return false
	}
}
