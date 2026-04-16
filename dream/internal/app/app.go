package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"dream/internal/agent"
	"dream/internal/claude"
	"dream/internal/codex"
	"dream/internal/doc"
	"dream/internal/state"
	dreamtemplate "dream/internal/template"
)

const defaultInterTaskDelay = 20 * time.Second

type Sleeper func(context.Context, time.Duration) error

type RunOptions struct {
	DocPath  string
	Model    string
	Settings string
}

type Controller struct {
	Store          *state.Store
	Executors      map[string]agent.Executor
	Cwd            string
	Out            io.Writer
	Err            io.Writer
	InterTaskDelay time.Duration
	Now            func() time.Time
	Sleep          Sleeper
}

func New(cwd string, out, err io.Writer) *Controller {
	return &Controller{
		Store: state.New(cwd),
		Executors: map[string]agent.Executor{
			agent.ModelCodex:  codex.New(out, err),
			agent.ModelClaude: claude.New(out, err),
		},
		Cwd:            cwd,
		Out:            out,
		Err:            err,
		InterTaskDelay: defaultInterTaskDelay,
		Now:            time.Now,
		Sleep:          sleepContext,
	}
}

func (c *Controller) Execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		c.printHelp()
		return nil
	}

	switch args[0] {
	case "run":
		return c.executeRun(ctx, args[1:])
	case "resume":
		return c.executeResume(ctx, args[1:])
	case "status":
		return c.executeStatus(args[1:])
	case "logs":
		return c.executeLogs(args[1:])
	case "template":
		return c.executeTemplate(args[1:])
	case "help", "-h", "--help":
		c.printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (c *Controller) RunWorkflow(ctx context.Context, opts RunOptions) error {
	model := agent.NormalizeModel(opts.Model)
	if !agent.IsSupportedModel(model) {
		return fmt.Errorf("unsupported model %q", opts.Model)
	}
	if model != agent.ModelClaude && strings.TrimSpace(opts.Settings) != "" {
		return fmt.Errorf("--settings is only supported with -m claude")
	}

	absDoc, err := resolveDocPath(opts.DocPath)
	if err != nil {
		return err
	}

	projectRoot, _, err := doc.EnsureProjectRoot(absDoc, c.Cwd, c.Err)
	if err != nil {
		return err
	}

	if err := c.Store.Ensure(); err != nil {
		return err
	}

	workflowID := state.WorkflowID(absDoc)
	releaseLock, err := c.Store.AcquireLock(workflowID)
	if err != nil {
		return err
	}
	defer releaseLock()

	meta := state.WorkflowMeta{
		WorkflowID:  workflowID,
		DocPath:     absDoc,
		ProjectRoot: projectRoot,
		Model:       model,
		Settings:    strings.TrimSpace(opts.Settings),
	}
	if err := c.Store.SaveMeta(meta); err != nil {
		return err
	}

	status, exists, err := c.loadOrCreateStatus(meta)
	if err != nil {
		return err
	}
	status.ProjectRoot = projectRoot
	status.DocPath = absDoc

	if exists {
		switch status.State {
		case state.StateRunning:
			return fmt.Errorf("workflow %s is already running", workflowID)
		case state.StateInterrupted:
			return fmt.Errorf("workflow %s is interrupted; use `dream resume %s`", workflowID, absDoc)
		}
	}

	c.logf("workflow %s starting from %s with model %s", workflowID, absDoc, meta.EffectiveModel())
	return c.runLoop(ctx, meta, &status, false)
}

func (c *Controller) ResumeWorkflow(ctx context.Context, ref string) error {
	if err := c.Store.Ensure(); err != nil {
		return err
	}

	meta, status, err := c.Store.Resolve(ref)
	if err != nil {
		return err
	}
	if status.State == state.StateCompleted {
		c.logf("workflow %s is already completed", status.WorkflowID)
		return nil
	}
	if status.CurrentRound == 0 {
		return fmt.Errorf("workflow %s has no round to resume", status.WorkflowID)
	}

	releaseLock, err := c.Store.AcquireLock(status.WorkflowID)
	if err != nil {
		return err
	}
	defer releaseLock()

	c.logf("workflow %s resuming round %04d with model %s", status.WorkflowID, status.CurrentRound, meta.EffectiveModel())
	return c.runLoop(ctx, meta, &status, true)
}

func (c *Controller) ShowStatus(ref string) error {
	if ref == "" {
		entries, err := c.Store.ListIndex()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			_, _ = fmt.Fprintln(c.Out, "no workflows")
			return nil
		}

		writer := tabwriter.NewWriter(c.Out, 0, 8, 2, ' ', 0)
		_, _ = fmt.Fprintln(writer, "WORKFLOW\tMODEL\tSTATE\tROUND\tSESSION\tDOC")
		for _, entry := range entries {
			_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%s\t%s\n", entry.WorkflowID, entry.Model, entry.State, entry.CurrentRound, entry.CurrentSessionID, entry.DocPath)
		}
		return writer.Flush()
	}

	meta, status, err := c.Store.Resolve(ref)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(c.Out, "workflow_id: %s\n", status.WorkflowID)
	_, _ = fmt.Fprintf(c.Out, "model: %s\n", meta.EffectiveModel())
	_, _ = fmt.Fprintf(c.Out, "state: %s\n", status.State)
	_, _ = fmt.Fprintf(c.Out, "current_round: %d\n", status.CurrentRound)
	_, _ = fmt.Fprintf(c.Out, "current_session_id: %s\n", status.CurrentSessionID)
	_, _ = fmt.Fprintf(c.Out, "doc_path: %s\n", meta.DocPath)
	_, _ = fmt.Fprintf(c.Out, "project_root: %s\n", meta.ProjectRoot)
	_, _ = fmt.Fprintf(c.Out, "created_at: %s\n", status.CreatedAt.Format(time.RFC3339))
	_, _ = fmt.Fprintf(c.Out, "updated_at: %s\n", status.UpdatedAt.Format(time.RFC3339))
	return nil
}

func (c *Controller) ShowLogs(ref string, round int) error {
	_, status, err := c.Store.Resolve(ref)
	if err != nil {
		return err
	}
	if round <= 0 {
		round = status.CurrentRound
	}
	if round <= 0 {
		return fmt.Errorf("workflow %s has no logs", status.WorkflowID)
	}

	paths := c.Store.RoundPaths(status.WorkflowID, round)
	data, err := os.ReadFile(paths.Stdout)
	if err != nil {
		return err
	}
	_, err = c.Out.Write(data)
	return err
}

func (c *Controller) CreateTemplate(dirArg string) error {
	targetDir, err := resolveTemplateDir(c.Cwd, dirArg)
	if err != nil {
		return err
	}

	result, err := dreamtemplate.Create(targetDir)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(c.Out, "created template in %s\n", result.Dir)
	_, _ = fmt.Fprintf(c.Out, "- %s\n", result.ReadmePath)
	_, _ = fmt.Fprintf(c.Out, "- %s\n", result.FeaturePath)
	return nil
}

func (c *Controller) executeRun(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	model := fs.String("model", agent.ModelCodex, "model backend to use (supported: codex, claude)")
	fs.StringVar(model, "m", agent.ModelCodex, "model backend to use (supported: codex, claude)")
	settings := fs.String("settings", "", "settings file path or JSON passed to claude --settings")
	fs.StringVar(settings, "s", "", "settings file path or JSON passed to claude --settings")
	fs.SetOutput(c.Err)
	fs.Usage = func() {
		c.printRunHelp(fs.Output())
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: dream run [--model codex|claude] [--settings path-or-json] <doc>")
	}
	return c.RunWorkflow(ctx, RunOptions{
		DocPath:  fs.Arg(0),
		Model:    *model,
		Settings: *settings,
	})
}

func (c *Controller) executeResume(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: dream resume <doc-or-workflow>")
	}
	return c.ResumeWorkflow(ctx, fs.Arg(0))
}

func (c *Controller) executeStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: dream status [doc-or-workflow]")
	}
	ref := ""
	if fs.NArg() == 1 {
		ref = fs.Arg(0)
	}
	return c.ShowStatus(ref)
}

func (c *Controller) executeLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	round := fs.Int("round", 0, "round number")
	fs.SetOutput(c.Err)
	normalizedArgs := normalizeLogsArgs(args)
	if err := fs.Parse(normalizedArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: dream logs <doc-or-workflow> [--round N]")
	}
	return c.ShowLogs(fs.Arg(0), *round)
}

func (c *Controller) executeTemplate(args []string) error {
	fs := flag.NewFlagSet("template", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	fs.Usage = func() {
		c.printTemplateHelp(fs.Output())
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: dream template <dir>")
	}
	return c.CreateTemplate(fs.Arg(0))
}

func (c *Controller) loadOrCreateStatus(meta state.WorkflowMeta) (state.WorkflowStatus, bool, error) {
	status, err := c.Store.LoadStatus(meta.WorkflowID)
	if err == nil {
		return status, true, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return state.WorkflowStatus{}, false, err
	}

	now := c.Now()
	status = state.WorkflowStatus{
		WorkflowID:  meta.WorkflowID,
		DocPath:     meta.DocPath,
		ProjectRoot: meta.ProjectRoot,
		State:       state.StateCreated,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := c.Store.SaveStatus(status); err != nil {
		return state.WorkflowStatus{}, false, err
	}
	return status, false, nil
}

func (c *Controller) runLoop(ctx context.Context, meta state.WorkflowMeta, status *state.WorkflowStatus, resumeCurrent bool) error {
	if resumeCurrent {
		shouldContinue, err := c.resumeCurrentRound(ctx, meta, status)
		if err != nil {
			return err
		}
		if !shouldContinue {
			return nil
		}
		if err := c.waitBetweenRounds(ctx, *status); err != nil {
			return c.markInterrupted(status, err)
		}
	}

	for {
		shouldContinue, err := c.runNewRound(ctx, meta, status)
		if err != nil {
			return err
		}
		if !shouldContinue {
			return nil
		}
		if err := c.waitBetweenRounds(ctx, *status); err != nil {
			return c.markInterrupted(status, err)
		}
	}
}

func (c *Controller) runNewRound(ctx context.Context, meta state.WorkflowMeta, status *state.WorkflowStatus) (bool, error) {
	round := status.CurrentRound + 1
	paths, err := c.Store.EnsureRound(meta.WorkflowID, round)
	if err != nil {
		return false, err
	}

	executor, err := c.executorForModel(meta.EffectiveModel())
	if err != nil {
		return false, c.markRoundFailure(status, err)
	}

	prompt := BuildRoundPrompt(meta.DocPath, meta.ProjectRoot, round)
	if err := os.WriteFile(paths.Prompt, []byte(prompt), 0o644); err != nil {
		return false, err
	}

	status.CurrentRound = round
	status.DocPath = meta.DocPath
	status.ProjectRoot = meta.ProjectRoot
	status.State = state.StateRunning
	status.CurrentSessionID = ""
	if err := c.saveStatus(status); err != nil {
		return false, err
	}

	c.logf("workflow %s round %04d: starting new %s session", meta.WorkflowID, round, meta.EffectiveModel())
	result, err := executor.Run(ctx, agent.RunRequest{
		ProjectRoot:     meta.ProjectRoot,
		Prompt:          prompt,
		LastMessagePath: paths.Last,
		StdoutLogPath:   paths.Stdout,
		StderrLogPath:   paths.Stderr,
		Settings:        meta.Settings,
	})
	if result.SessionID != "" {
		if writeErr := c.writeSessionID(paths.SessionID, result.SessionID); writeErr != nil {
			return false, writeErr
		}
		status.CurrentSessionID = result.SessionID
		if saveErr := c.saveStatus(status); saveErr != nil {
			return false, saveErr
		}
	}
	if err != nil {
		if ctx.Err() != nil {
			return false, c.markInterrupted(status, ctx.Err())
		}
		return false, c.markRoundFailure(status, err)
	}
	if result.SessionID == "" {
		return false, c.markRoundFailure(status, fmt.Errorf("round %04d did not emit a session id", round))
	}
	return c.finishRound(status, result.LastMessage)
}

func (c *Controller) resumeCurrentRound(ctx context.Context, meta state.WorkflowMeta, status *state.WorkflowStatus) (bool, error) {
	paths := c.Store.RoundPaths(meta.WorkflowID, status.CurrentRound)
	sessionID := status.CurrentSessionID
	if sessionID == "" {
		data, err := os.ReadFile(paths.SessionID)
		if err != nil {
			return false, err
		}
		sessionID = strings.TrimSpace(string(data))
	}
	if sessionID == "" {
		return false, fmt.Errorf("workflow %s round %04d has no session id", meta.WorkflowID, status.CurrentRound)
	}

	status.State = state.StateRunning
	status.ProjectRoot = meta.ProjectRoot
	status.DocPath = meta.DocPath
	status.CurrentSessionID = sessionID
	if err := c.saveStatus(status); err != nil {
		return false, err
	}

	executor, err := c.executorForModel(meta.EffectiveModel())
	if err != nil {
		return false, c.markRoundFailure(status, err)
	}

	result, err := executor.Resume(ctx, agent.ResumeRequest{
		SessionID:       sessionID,
		Prompt:          BuildResumePrompt(status.CurrentRound),
		ProjectRoot:     meta.ProjectRoot,
		LastMessagePath: paths.Last,
		StdoutLogPath:   paths.Stdout,
		StderrLogPath:   paths.Stderr,
		Settings:        meta.Settings,
	})
	if result.SessionID == "" {
		result.SessionID = sessionID
	}
	if err != nil {
		if ctx.Err() != nil {
			return false, c.markInterrupted(status, ctx.Err())
		}
		return false, c.markRoundFailure(status, err)
	}
	return c.finishRound(status, result.LastMessage)
}

func (c *Controller) finishRound(status *state.WorkflowStatus, lastMessage string) (bool, error) {
	outcome, err := ParseRoundOutcome(lastMessage)
	if err != nil {
		return false, c.markRoundFailure(status, fmt.Errorf("round %04d final message protocol error: %w", status.CurrentRound, err))
	}

	if outcome.AllDone {
		status.State = state.StateCompleted
		if err := c.saveStatus(status); err != nil {
			return false, err
		}
		c.logf("workflow %s completed at round %04d", status.WorkflowID, status.CurrentRound)
		return false, nil
	}

	status.State = state.StateRunning
	if err := c.saveStatus(status); err != nil {
		return false, err
	}
	c.logf("workflow %s round %04d complete: %s", status.WorkflowID, status.CurrentRound, outcome.TaskDone)
	return true, nil
}

func (c *Controller) waitBetweenRounds(ctx context.Context, status state.WorkflowStatus) error {
	c.logf("workflow %s sleeping %s before next round", status.WorkflowID, c.InterTaskDelay)
	return c.Sleep(ctx, c.InterTaskDelay)
}

func (c *Controller) markRoundFailure(status *state.WorkflowStatus, cause error) error {
	status.State = state.StateFailed
	if errors.Is(cause, context.Canceled) {
		status.State = state.StateInterrupted
	}
	if err := c.saveStatus(status); err != nil {
		return err
	}
	return cause
}

func (c *Controller) markInterrupted(status *state.WorkflowStatus, cause error) error {
	status.State = state.StateInterrupted
	if err := c.saveStatus(status); err != nil {
		return err
	}
	return cause
}

func (c *Controller) saveStatus(status *state.WorkflowStatus) error {
	now := c.Now()
	if status.CreatedAt.IsZero() {
		status.CreatedAt = now
	}
	status.UpdatedAt = now
	return c.Store.SaveStatus(*status)
}

func (c *Controller) writeSessionID(path, sessionID string) error {
	return os.WriteFile(path, []byte(sessionID+"\n"), 0o644)
}

func (c *Controller) executorForModel(model string) (agent.Executor, error) {
	normalized := agent.NormalizeModel(model)
	executor := c.Executors[normalized]
	if executor == nil {
		return nil, fmt.Errorf("model %q is not configured", normalized)
	}
	return executor, nil
}

func (c *Controller) logf(format string, args ...any) {
	if c.Err == nil {
		return
	}
	_, _ = fmt.Fprintf(c.Err, "[dream] "+format+"\n", args...)
}

func (c *Controller) printHelp() {
	_, _ = fmt.Fprintln(c.Out, "dream")
	_, _ = fmt.Fprintln(c.Out, "Drive the same workflow document with codex or claude in rounds.")
	_, _ = fmt.Fprintln(c.Out, "")
	_, _ = fmt.Fprintln(c.Out, "Usage:")
	_, _ = fmt.Fprintln(c.Out, "  dream <command> [options]")
	_, _ = fmt.Fprintln(c.Out, "")
	_, _ = fmt.Fprintln(c.Out, "Commands:")

	writer := tabwriter.NewWriter(c.Out, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "  run\tstart a workflow from a document")
	_, _ = fmt.Fprintln(writer, "  resume\tresume an interrupted workflow")
	_, _ = fmt.Fprintln(writer, "  status\tshow workflow status")
	_, _ = fmt.Fprintln(writer, "  logs\tshow logs for the current or specified round")
	_, _ = fmt.Fprintln(writer, "  template\tgenerate a workflow document template directory")
	_, _ = fmt.Fprintln(writer, "  help\tshow this help")
	_ = writer.Flush()

	_, _ = fmt.Fprintln(c.Out, "")
	_, _ = fmt.Fprintln(c.Out, "Supported models:")

	writer = tabwriter.NewWriter(c.Out, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "  codex\tdefault backend")
	_, _ = fmt.Fprintln(writer, "  claude\tClaude backend; supports --settings")
	_ = writer.Flush()

	_, _ = fmt.Fprintln(c.Out, "")
	_, _ = fmt.Fprintln(c.Out, "Examples:")
	_, _ = fmt.Fprintln(c.Out, "  dream run ./task.md")
	_, _ = fmt.Fprintln(c.Out, "  dream run -m claude ./task.md")
	_, _ = fmt.Fprintln(c.Out, "  dream run -m claude -s ~/.claude/settings.json ./task.md")
	_, _ = fmt.Fprintln(c.Out, "  dream resume ./task.md")
	_, _ = fmt.Fprintln(c.Out, "  dream status ./task.md")
	_, _ = fmt.Fprintln(c.Out, "  dream logs ./task.md --round 2")
	_, _ = fmt.Fprintln(c.Out, "  dream template my-plan")
	_, _ = fmt.Fprintln(c.Out, "")
	_, _ = fmt.Fprintln(c.Out, "Use \"dream run -h\" or \"dream template -h\" for command-specific help.")
}

func (c *Controller) printRunHelp(out io.Writer) {
	_, _ = fmt.Fprintln(out, "Usage:")
	_, _ = fmt.Fprintln(out, "  dream run [--model codex|claude] [--settings path-or-json] <doc>")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Arguments:")

	writer := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "  <doc>\tworkflow document path")
	_ = writer.Flush()

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Options:")

	writer = tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "  -m, --model\tbackend to use; supported: codex, claude (default codex)")
	_, _ = fmt.Fprintln(writer, "  -s, --settings\tsettings file path or JSON passed to claude --settings")
	_ = writer.Flush()

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Examples:")
	_, _ = fmt.Fprintln(out, "  dream run ./task.md")
	_, _ = fmt.Fprintln(out, "  dream run -m claude ./task.md")
	_, _ = fmt.Fprintln(out, "  dream run -m claude -s ~/.claude/settings.json ./task.md")
}

func (c *Controller) printTemplateHelp(out io.Writer) {
	_, _ = fmt.Fprintln(out, "Usage:")
	_, _ = fmt.Fprintln(out, "  dream template <dir>")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Arguments:")

	writer := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "  <dir>\ttarget directory to create under the current working directory")
	_ = writer.Flush()

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Behavior:")
	_, _ = fmt.Fprintln(out, "  Creates a workflow template directory with README.md and a sample feature document.")
	_, _ = fmt.Fprintln(out, "  The target directory must not already contain files.")

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Examples:")
	_, _ = fmt.Fprintln(out, "  dream template my-plan")
	_, _ = fmt.Fprintln(out, "  dream template plans/refactor")
}

func resolveDocPath(docArg string) (string, error) {
	if docArg == "" {
		return "", fmt.Errorf("missing document path")
	}
	absDoc, err := filepath.Abs(docArg)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absDoc)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", absDoc)
	}
	return absDoc, nil
}

func resolveTemplateDir(cwd, dirArg string) (string, error) {
	if strings.TrimSpace(dirArg) == "" {
		return "", fmt.Errorf("missing template directory")
	}

	if filepath.IsAbs(dirArg) {
		return filepath.Clean(dirArg), nil
	}
	return filepath.Join(cwd, dirArg), nil
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func normalizeLogsArgs(args []string) []string {
	if len(args) <= 1 {
		return args
	}

	normalized := make([]string, 0, len(args))
	positional := make([]string, 0, 1)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--round":
			if i+1 < len(args) {
				normalized = append(normalized, args[i], args[i+1])
				i++
				continue
			}
		default:
			if strings.HasPrefix(args[i], "--round=") {
				normalized = append(normalized, args[i])
				continue
			}
		}
		positional = append(positional, args[i])
	}

	return append(normalized, positional...)
}
