package app

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dream/internal/agent"
	"dream/internal/state"
)

type fakeExecutor struct {
	runFn    func(context.Context, agent.RunRequest) (agent.Result, error)
	resumeFn func(context.Context, agent.ResumeRequest) (agent.Result, error)
}

func (f fakeExecutor) Run(ctx context.Context, req agent.RunRequest) (agent.Result, error) {
	if f.runFn == nil {
		return agent.Result{}, errors.New("unexpected run")
	}
	return f.runFn(ctx, req)
}

func (f fakeExecutor) Resume(ctx context.Context, req agent.ResumeRequest) (agent.Result, error) {
	if f.resumeFn == nil {
		return agent.Result{}, errors.New("unexpected resume")
	}
	return f.resumeFn(ctx, req)
}

func TestRunWorkflowCreatesNewSessionPerRoundAndSleeps(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	docContent := "PROJECT_ROOT=" + projectRoot + "\n\n# Plan\n- task a\n- task b\n"
	if err := os.WriteFile(docPath, []byte(docContent), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	var sleepCalls []time.Duration
	runCount := 0
	controller := &Controller{
		Store: state.New(tempDir),
		Executors: map[string]agent.Executor{
			agent.ModelCodex: fakeExecutor{
				runFn: func(_ context.Context, req agent.RunRequest) (agent.Result, error) {
					runCount++
					if runCount == 1 {
						if !strings.Contains(req.Prompt, "当前轮次：1") {
							t.Fatalf("round 1 prompt missing round marker: %q", req.Prompt)
						}
						return agent.Result{
							SessionID:   "session-1",
							LastMessage: "TASK_DONE: first\nNEXT_TASK: second\n",
						}, nil
					}
					if runCount == 2 {
						if !strings.Contains(req.Prompt, "当前轮次：2") {
							t.Fatalf("round 2 prompt missing round marker: %q", req.Prompt)
						}
						return agent.Result{
							SessionID:   "session-2",
							LastMessage: "ALL_TASKS_DONE\n",
						}, nil
					}
					t.Fatalf("unexpected run count %d", runCount)
					return agent.Result{}, nil
				},
			},
		},
		Cwd:            tempDir,
		Out:            &bytes.Buffer{},
		Err:            &bytes.Buffer{},
		InterTaskDelay: 20 * time.Second,
		Now:            time.Now,
		Sleep: func(_ context.Context, delay time.Duration) error {
			sleepCalls = append(sleepCalls, delay)
			return nil
		},
	}

	if err := controller.RunWorkflow(context.Background(), RunOptions{
		DocPath: docPath,
		Model:   agent.ModelCodex,
	}); err != nil {
		t.Fatalf("RunWorkflow returned error: %v", err)
	}
	if runCount != 2 {
		t.Fatalf("expected 2 runs, got %d", runCount)
	}
	if len(sleepCalls) != 1 || sleepCalls[0] != 20*time.Second {
		t.Fatalf("unexpected sleep calls: %#v", sleepCalls)
	}

	workflowID := state.WorkflowID(docPath)
	status, err := controller.Store.LoadStatus(workflowID)
	if err != nil {
		t.Fatalf("LoadStatus returned error: %v", err)
	}
	if status.State != state.StateCompleted {
		t.Fatalf("unexpected final state: %s", status.State)
	}
	if status.CurrentRound != 2 {
		t.Fatalf("unexpected current round: %d", status.CurrentRound)
	}
	if status.CurrentSessionID != "session-2" {
		t.Fatalf("unexpected current session: %s", status.CurrentSessionID)
	}

	meta, err := controller.Store.LoadMeta(workflowID)
	if err != nil {
		t.Fatalf("LoadMeta returned error: %v", err)
	}
	if meta.EffectiveModel() != agent.ModelCodex {
		t.Fatalf("unexpected model: %s", meta.EffectiveModel())
	}

	round1 := controller.Store.RoundPaths(workflowID, 1)
	round2 := controller.Store.RoundPaths(workflowID, 2)

	session1, err := os.ReadFile(round1.SessionID)
	if err != nil {
		t.Fatalf("read round1 session id: %v", err)
	}
	session2, err := os.ReadFile(round2.SessionID)
	if err != nil {
		t.Fatalf("read round2 session id: %v", err)
	}
	if strings.TrimSpace(string(session1)) != "session-1" || strings.TrimSpace(string(session2)) != "session-2" {
		t.Fatalf("unexpected session history: %q %q", string(session1), string(session2))
	}
}

func TestRunWorkflowUsesControllerCwdWhenProjectRootMissing(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(docPath, []byte("# Plan\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	controller := &Controller{
		Store: state.New(tempDir),
		Cwd:   projectRoot,
		Executors: map[string]agent.Executor{
			agent.ModelCodex: fakeExecutor{
				runFn: func(_ context.Context, req agent.RunRequest) (agent.Result, error) {
					if req.ProjectRoot != projectRoot {
						t.Fatalf("unexpected project root: %s", req.ProjectRoot)
					}
					return agent.Result{
						SessionID:   "session-1",
						LastMessage: "ALL_TASKS_DONE\n",
					}, nil
				},
			},
		},
		Out:            &bytes.Buffer{},
		Err:            &bytes.Buffer{},
		InterTaskDelay: 20 * time.Second,
		Now:            time.Now,
		Sleep: func(_ context.Context, delay time.Duration) error {
			return nil
		},
	}

	if err := controller.RunWorkflow(context.Background(), RunOptions{
		DocPath: docPath,
		Model:   agent.ModelCodex,
	}); err != nil {
		t.Fatalf("RunWorkflow returned error: %v", err)
	}

	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	if !strings.HasPrefix(string(content), "PROJECT_ROOT="+projectRoot+"\n\n") {
		t.Fatalf("document head not updated correctly:\n%s", string(content))
	}
}

func TestRunWorkflowUsesClaudeExecutorAndPersistsSettings(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(docPath, []byte("PROJECT_ROOT="+projectRoot+"\n\n# Plan\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	controller := &Controller{
		Store: state.New(tempDir),
		Executors: map[string]agent.Executor{
			agent.ModelClaude: fakeExecutor{
				runFn: func(_ context.Context, req agent.RunRequest) (agent.Result, error) {
					if req.Settings != "~/.claude/settings-ice.json" {
						t.Fatalf("unexpected settings: %q", req.Settings)
					}
					return agent.Result{
						SessionID:   "claude-session-1",
						LastMessage: "ALL_TASKS_DONE\n",
					}, nil
				},
			},
		},
		Cwd:            tempDir,
		Out:            &bytes.Buffer{},
		Err:            &bytes.Buffer{},
		InterTaskDelay: 20 * time.Second,
		Now:            time.Now,
		Sleep: func(_ context.Context, delay time.Duration) error {
			return nil
		},
	}

	if err := controller.RunWorkflow(context.Background(), RunOptions{
		DocPath:  docPath,
		Model:    "CLAUDE",
		Settings: "~/.claude/settings-ice.json",
	}); err != nil {
		t.Fatalf("RunWorkflow returned error: %v", err)
	}

	meta, err := controller.Store.LoadMeta(state.WorkflowID(docPath))
	if err != nil {
		t.Fatalf("LoadMeta returned error: %v", err)
	}
	if meta.EffectiveModel() != agent.ModelClaude {
		t.Fatalf("unexpected model: %s", meta.EffectiveModel())
	}
	if meta.Settings != "~/.claude/settings-ice.json" {
		t.Fatalf("unexpected settings: %q", meta.Settings)
	}
}

func TestResumeWorkflowResumesCurrentRoundThenStartsNextRound(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(docPath, []byte("PROJECT_ROOT="+projectRoot+"\n\n# Plan\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	store := state.New(tempDir)
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure store: %v", err)
	}

	workflowID := state.WorkflowID(docPath)
	meta := state.WorkflowMeta{
		WorkflowID:  workflowID,
		DocPath:     docPath,
		ProjectRoot: projectRoot,
		Model:       agent.ModelCodex,
	}
	if err := store.SaveMeta(meta); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}

	now := time.Now()
	status := state.WorkflowStatus{
		WorkflowID:       workflowID,
		DocPath:          docPath,
		ProjectRoot:      projectRoot,
		State:            state.StateInterrupted,
		CurrentRound:     1,
		CurrentSessionID: "session-1",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := store.SaveStatus(status); err != nil {
		t.Fatalf("SaveStatus: %v", err)
	}

	round1, err := store.EnsureRound(workflowID, 1)
	if err != nil {
		t.Fatalf("EnsureRound: %v", err)
	}
	if err := os.WriteFile(round1.SessionID, []byte("session-1\n"), 0o644); err != nil {
		t.Fatalf("write session id: %v", err)
	}

	var sleepCalls int
	var resumeCalls int
	var runCalls int
	controller := &Controller{
		Store: store,
		Executors: map[string]agent.Executor{
			agent.ModelCodex: fakeExecutor{
				resumeFn: func(_ context.Context, req agent.ResumeRequest) (agent.Result, error) {
					resumeCalls++
					if req.SessionID != "session-1" {
						t.Fatalf("unexpected resume session id: %s", req.SessionID)
					}
					return agent.Result{
						SessionID:   "session-1",
						LastMessage: "TASK_DONE: resumed first\nNEXT_TASK: second\n",
					}, nil
				},
				runFn: func(_ context.Context, req agent.RunRequest) (agent.Result, error) {
					runCalls++
					if !strings.Contains(req.Prompt, "当前轮次：2") {
						t.Fatalf("round 2 prompt missing round marker: %q", req.Prompt)
					}
					return agent.Result{
						SessionID:   "session-2",
						LastMessage: "ALL_TASKS_DONE\n",
					}, nil
				},
			},
		},
		Cwd:            tempDir,
		Out:            &bytes.Buffer{},
		Err:            &bytes.Buffer{},
		InterTaskDelay: 20 * time.Second,
		Now:            time.Now,
		Sleep: func(_ context.Context, delay time.Duration) error {
			if delay != 20*time.Second {
				t.Fatalf("unexpected delay: %s", delay)
			}
			sleepCalls++
			return nil
		},
	}

	if err := controller.ResumeWorkflow(context.Background(), docPath); err != nil {
		t.Fatalf("ResumeWorkflow returned error: %v", err)
	}
	if resumeCalls != 1 {
		t.Fatalf("expected 1 resume call, got %d", resumeCalls)
	}
	if runCalls != 1 {
		t.Fatalf("expected 1 new run call, got %d", runCalls)
	}
	if sleepCalls != 1 {
		t.Fatalf("expected 1 sleep call, got %d", sleepCalls)
	}

	finalStatus, err := store.LoadStatus(workflowID)
	if err != nil {
		t.Fatalf("LoadStatus returned error: %v", err)
	}
	if finalStatus.State != state.StateCompleted {
		t.Fatalf("unexpected final state: %s", finalStatus.State)
	}
	if finalStatus.CurrentRound != 2 {
		t.Fatalf("unexpected current round: %d", finalStatus.CurrentRound)
	}
}

func TestResumeWorkflowDefaultsMissingModelToCodex(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(docPath, []byte("PROJECT_ROOT="+projectRoot+"\n\n# Plan\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	store := state.New(tempDir)
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure store: %v", err)
	}

	workflowID := state.WorkflowID(docPath)
	if err := store.SaveMeta(state.WorkflowMeta{
		WorkflowID:  workflowID,
		DocPath:     docPath,
		ProjectRoot: projectRoot,
	}); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}

	now := time.Now()
	if err := store.SaveStatus(state.WorkflowStatus{
		WorkflowID:       workflowID,
		DocPath:          docPath,
		ProjectRoot:      projectRoot,
		State:            state.StateInterrupted,
		CurrentRound:     1,
		CurrentSessionID: "legacy-session-1",
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		t.Fatalf("SaveStatus: %v", err)
	}

	round1, err := store.EnsureRound(workflowID, 1)
	if err != nil {
		t.Fatalf("EnsureRound: %v", err)
	}
	if err := os.WriteFile(round1.SessionID, []byte("legacy-session-1\n"), 0o644); err != nil {
		t.Fatalf("write session id: %v", err)
	}

	controller := &Controller{
		Store: store,
		Executors: map[string]agent.Executor{
			agent.ModelCodex: fakeExecutor{
				resumeFn: func(_ context.Context, req agent.ResumeRequest) (agent.Result, error) {
					if req.SessionID != "legacy-session-1" {
						t.Fatalf("unexpected session id: %s", req.SessionID)
					}
					return agent.Result{
						SessionID:   "legacy-session-1",
						LastMessage: "ALL_TASKS_DONE\n",
					}, nil
				},
			},
		},
		Cwd:            tempDir,
		Out:            &bytes.Buffer{},
		Err:            &bytes.Buffer{},
		InterTaskDelay: 20 * time.Second,
		Now:            time.Now,
		Sleep: func(_ context.Context, delay time.Duration) error {
			return nil
		},
	}

	if err := controller.ResumeWorkflow(context.Background(), docPath); err != nil {
		t.Fatalf("ResumeWorkflow returned error: %v", err)
	}
}

func TestExecuteRunRejectsSettingsForCodex(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(docPath, []byte("PROJECT_ROOT="+projectRoot+"\n\n# Plan\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	controller := &Controller{
		Store:     state.New(tempDir),
		Executors: map[string]agent.Executor{},
		Cwd:       tempDir,
		Out:       &bytes.Buffer{},
		Err:       &bytes.Buffer{},
	}

	err := controller.Execute(context.Background(), []string{"run", "-s", "~/.claude/settings.json", docPath})
	if err == nil || !strings.Contains(err.Error(), "--settings is only supported with -m claude") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteTopLevelHelpMentionsClaude(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	controller := &Controller{
		Store:     state.New(t.TempDir()),
		Executors: map[string]agent.Executor{},
		Cwd:       t.TempDir(),
		Out:       &out,
		Err:       &errOut,
	}

	if err := controller.Execute(context.Background(), []string{"-h"}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	help := out.String()
	if !strings.Contains(help, "Drive the same workflow document with codex or claude in rounds.") {
		t.Fatalf("top-level help missing overview: %q", help)
	}
	if !strings.Contains(help, "claude") {
		t.Fatalf("top-level help missing Claude model info: %q", help)
	}
	if !strings.Contains(help, "dream run -m claude ./task.md") {
		t.Fatalf("top-level help missing Claude example: %q", help)
	}
}

func TestExecuteRunHelpReturnsNilAndMentionsClaude(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	controller := &Controller{
		Store:     state.New(t.TempDir()),
		Executors: map[string]agent.Executor{},
		Cwd:       t.TempDir(),
		Out:       &out,
		Err:       &errOut,
	}

	err := controller.Execute(context.Background(), []string{"run", "-h"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected help to return nil, got %v", err)
	}
	help := errOut.String()
	if !strings.Contains(help, "dream run [--model codex|claude] [--settings path-or-json] <doc>") {
		t.Fatalf("run help missing usage: %q", help)
	}
	if !strings.Contains(help, "supported: codex, claude") {
		t.Fatalf("run help missing Claude model info: %q", help)
	}
	if !strings.Contains(help, "dream run -m claude ./task.md") {
		t.Fatalf("run help missing Claude example: %q", help)
	}
}

func TestShowStatusIncludesModel(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(docPath, []byte("PROJECT_ROOT="+projectRoot+"\n\n# Plan\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	store := state.New(tempDir)
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure store: %v", err)
	}

	workflowID := state.WorkflowID(docPath)
	if err := store.SaveMeta(state.WorkflowMeta{
		WorkflowID:  workflowID,
		DocPath:     docPath,
		ProjectRoot: projectRoot,
		Model:       agent.ModelClaude,
	}); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}

	now := time.Now()
	if err := store.SaveStatus(state.WorkflowStatus{
		WorkflowID:       workflowID,
		DocPath:          docPath,
		ProjectRoot:      projectRoot,
		State:            state.StateRunning,
		CurrentRound:     1,
		CurrentSessionID: "session-1",
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		t.Fatalf("SaveStatus: %v", err)
	}

	var out bytes.Buffer
	controller := &Controller{
		Store: store,
		Out:   &out,
		Err:   &bytes.Buffer{},
	}

	if err := controller.ShowStatus(docPath); err != nil {
		t.Fatalf("ShowStatus returned error: %v", err)
	}
	if !strings.Contains(out.String(), "model: claude") {
		t.Fatalf("status output missing model: %q", out.String())
	}
}

func TestRunWorkflowMarksInterruptedWhenContextCanceled(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	docPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(docPath, []byte("PROJECT_ROOT="+projectRoot+"\n\n# Plan\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	controller := &Controller{
		Store: state.New(tempDir),
		Executors: map[string]agent.Executor{
			agent.ModelCodex: fakeExecutor{
				runFn: func(_ context.Context, _ agent.RunRequest) (agent.Result, error) {
					return agent.Result{SessionID: "session-1"}, errors.New("signal: killed")
				},
			},
		},
		Cwd:            tempDir,
		Out:            &bytes.Buffer{},
		Err:            &bytes.Buffer{},
		InterTaskDelay: 20 * time.Second,
		Now:            time.Now,
		Sleep: func(_ context.Context, delay time.Duration) error {
			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := controller.RunWorkflow(ctx, RunOptions{
		DocPath: docPath,
		Model:   agent.ModelCodex,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	status, loadErr := controller.Store.LoadStatus(state.WorkflowID(docPath))
	if loadErr != nil {
		t.Fatalf("LoadStatus returned error: %v", loadErr)
	}
	if status.State != state.StateInterrupted {
		t.Fatalf("unexpected final state: %s", status.State)
	}
}
