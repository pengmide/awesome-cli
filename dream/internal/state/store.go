package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"dream/internal/agent"
)

const (
	StateCreated     = "created"
	StateRunning     = "running"
	StateInterrupted = "interrupted"
	StateCompleted   = "completed"
	StateFailed      = "failed"
)

type WorkflowMeta struct {
	WorkflowID  string `json:"workflow_id"`
	DocPath     string `json:"doc_path"`
	ProjectRoot string `json:"project_root"`
	Model       string `json:"model,omitempty"`
	Settings    string `json:"settings,omitempty"`
}

type WorkflowStatus struct {
	WorkflowID       string    `json:"workflow_id"`
	DocPath          string    `json:"doc_path"`
	ProjectRoot      string    `json:"project_root"`
	State            string    `json:"state"`
	CurrentRound     int       `json:"current_round"`
	CurrentSessionID string    `json:"current_session_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type IndexEntry struct {
	WorkflowID       string    `json:"workflow_id"`
	DocPath          string    `json:"doc_path"`
	ProjectRoot      string    `json:"project_root"`
	Model            string    `json:"model,omitempty"`
	State            string    `json:"state"`
	CurrentRound     int       `json:"current_round"`
	CurrentSessionID string    `json:"current_session_id"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type RoundPaths struct {
	Dir       string
	SessionID string
	Prompt    string
	Last      string
	Stdout    string
	Stderr    string
}

type indexFile struct {
	Workflows []IndexEntry `json:"workflows"`
}

type Store struct {
	Root string
}

func New(cwd string) *Store {
	return &Store{Root: filepath.Join(cwd, ".dream")}
}

func WorkflowID(absDocPath string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(absDocPath)))
	return hex.EncodeToString(sum[:8])
}

func (s *Store) Ensure() error {
	return os.MkdirAll(filepath.Join(s.Root, "workflows"), 0o755)
}

func (s *Store) WorkflowDir(workflowID string) string {
	return filepath.Join(s.Root, "workflows", workflowID)
}

func (s *Store) RoundPaths(workflowID string, round int) RoundPaths {
	dir := filepath.Join(s.WorkflowDir(workflowID), "rounds", fmt.Sprintf("%04d", round))
	return RoundPaths{
		Dir:       dir,
		SessionID: filepath.Join(dir, "session_id.txt"),
		Prompt:    filepath.Join(dir, "prompt.txt"),
		Last:      filepath.Join(dir, "last.txt"),
		Stdout:    filepath.Join(dir, "stdout.log"),
		Stderr:    filepath.Join(dir, "stderr.log"),
	}
}

func (s *Store) EnsureWorkflow(workflowID string) error {
	return os.MkdirAll(filepath.Join(s.WorkflowDir(workflowID), "rounds"), 0o755)
}

func (s *Store) EnsureRound(workflowID string, round int) (RoundPaths, error) {
	paths := s.RoundPaths(workflowID, round)
	if err := os.MkdirAll(paths.Dir, 0o755); err != nil {
		return RoundPaths{}, err
	}
	return paths, nil
}

func (s *Store) AcquireLock(workflowID string) (func() error, error) {
	if err := s.EnsureWorkflow(workflowID); err != nil {
		return nil, err
	}

	lockPath := filepath.Join(s.WorkflowDir(workflowID), "lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("workflow %s is already locked", workflowID)
		}
		return nil, err
	}

	if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
		_ = file.Close()
		_ = os.Remove(lockPath)
		return nil, err
	}

	return func() error {
		closeErr := file.Close()
		removeErr := os.Remove(lockPath)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
		return closeErr
	}, nil
}

func (s *Store) SaveMeta(meta WorkflowMeta) error {
	if err := s.EnsureWorkflow(meta.WorkflowID); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.WorkflowDir(meta.WorkflowID), "meta.json"), meta)
}

func (s *Store) LoadMeta(workflowID string) (WorkflowMeta, error) {
	var meta WorkflowMeta
	err := readJSON(filepath.Join(s.WorkflowDir(workflowID), "meta.json"), &meta)
	if err == nil {
		meta.Model = meta.EffectiveModel()
	}
	return meta, err
}

func (s *Store) SaveStatus(status WorkflowStatus) error {
	if err := s.EnsureWorkflow(status.WorkflowID); err != nil {
		return err
	}

	if err := writeJSON(filepath.Join(s.WorkflowDir(status.WorkflowID), "status.json"), status); err != nil {
		return err
	}
	return s.updateIndex(status)
}

func (s *Store) LoadStatus(workflowID string) (WorkflowStatus, error) {
	var status WorkflowStatus
	err := readJSON(filepath.Join(s.WorkflowDir(workflowID), "status.json"), &status)
	return status, err
}

func (s *Store) ListIndex() ([]IndexEntry, error) {
	var idx indexFile
	if err := readJSON(filepath.Join(s.Root, "index.json"), &idx); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	for i := range idx.Workflows {
		idx.Workflows[i].Model = agent.NormalizeModel(idx.Workflows[i].Model)
	}

	slices.SortFunc(idx.Workflows, func(a, b IndexEntry) int {
		switch {
		case a.UpdatedAt.After(b.UpdatedAt):
			return -1
		case a.UpdatedAt.Before(b.UpdatedAt):
			return 1
		default:
			return strings.Compare(a.WorkflowID, b.WorkflowID)
		}
	})
	return idx.Workflows, nil
}

func (s *Store) Resolve(ref string) (WorkflowMeta, WorkflowStatus, error) {
	if ref == "" {
		return WorkflowMeta{}, WorkflowStatus{}, fmt.Errorf("missing workflow reference")
	}

	candidates := []string{ref}
	if !filepath.IsAbs(ref) {
		if abs, err := filepath.Abs(ref); err == nil {
			candidates = append(candidates, abs)
		}
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			absCandidate, err := filepath.Abs(candidate)
			if err != nil {
				return WorkflowMeta{}, WorkflowStatus{}, err
			}
			workflowID := WorkflowID(absCandidate)
			return s.loadByID(workflowID)
		}
	}

	return s.loadByID(ref)
}

func (s *Store) updateIndex(status WorkflowStatus) error {
	if err := s.Ensure(); err != nil {
		return err
	}

	indexPath := filepath.Join(s.Root, "index.json")
	var idx indexFile
	if err := readJSON(indexPath, &idx); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	entry := IndexEntry{
		WorkflowID:       status.WorkflowID,
		DocPath:          status.DocPath,
		ProjectRoot:      status.ProjectRoot,
		Model:            agent.ModelCodex,
		State:            status.State,
		CurrentRound:     status.CurrentRound,
		CurrentSessionID: status.CurrentSessionID,
		UpdatedAt:        status.UpdatedAt,
	}

	meta, err := s.LoadMeta(status.WorkflowID)
	switch {
	case err == nil:
		entry.Model = meta.EffectiveModel()
	case errors.Is(err, os.ErrNotExist):
		entry.Model = agent.ModelCodex
	default:
		return err
	}

	replaced := false
	for i := range idx.Workflows {
		if idx.Workflows[i].WorkflowID == status.WorkflowID {
			idx.Workflows[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		idx.Workflows = append(idx.Workflows, entry)
	}

	return writeJSON(indexPath, idx)
}

func (s *Store) loadByID(workflowID string) (WorkflowMeta, WorkflowStatus, error) {
	meta, err := s.LoadMeta(workflowID)
	if err != nil {
		return WorkflowMeta{}, WorkflowStatus{}, err
	}
	status, err := s.LoadStatus(workflowID)
	if err != nil {
		return WorkflowMeta{}, WorkflowStatus{}, err
	}
	return meta, status, nil
}

func (m WorkflowMeta) EffectiveModel() string {
	return agent.NormalizeModel(m.Model)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data, 0o644)
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()

	cleanup := func(cause error) error {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return cause
	}

	if _, err := temp.Write(data); err != nil {
		return cleanup(err)
	}
	if err := temp.Chmod(mode); err != nil {
		return cleanup(err)
	}
	if err := temp.Close(); err != nil {
		return cleanup(err)
	}
	return os.Rename(tempName, path)
}
