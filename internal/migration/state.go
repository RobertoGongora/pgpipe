package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pgpipe/pgpipe/internal/config"
	"gopkg.in/yaml.v3"
)

// State represents the migration progress state
type State struct {
	ConfigHash string        `yaml:"config_hash"`
	Session    SessionState  `yaml:"session"`
	Source     SourceState   `yaml:"source"`
	Progress   ProgressState `yaml:"progress"`
	Batches    BatchState    `yaml:"batches"`
	LastRun    LastRunState  `yaml:"last_run"`

	// statePath overrides the default .pgpipe/state.yaml save location.
	// Not serialised — set at runtime by the CLI when using --config=<path>.
	statePath string `yaml:"-"`
}

// SessionState holds session-specific information
type SessionState struct {
	ID        string    `yaml:"id"`
	StartedAt time.Time `yaml:"started_at"`
	ErrorLog  string    `yaml:"error_log"`
}

// SourceState holds information about the source table at migration start
type SourceState struct {
	Table      string `yaml:"table"`
	TotalRows  int64  `yaml:"total_rows"`
	PrimaryKey string `yaml:"primary_key"`
	MinID      int64  `yaml:"min_id"`
	MaxID      int64  `yaml:"max_id"`
}

// ProgressState tracks overall migration progress
type ProgressState struct {
	LastCursor    int64 `yaml:"last_cursor"`
	ProcessedRows int64 `yaml:"processed_rows"`
	ImportedRows  int64 `yaml:"imported_rows"`
	SkippedRows   int64 `yaml:"skipped_rows"`
}

// BatchState tracks batch-level progress
type BatchState struct {
	Size      int `yaml:"size"`
	Completed int `yaml:"completed"`
}

// LastRunState holds information about the last run
type LastRunState struct {
	Mode             string    `yaml:"mode"` // "batches" or "continuous"
	BatchesRequested int       `yaml:"batches_requested,omitempty"`
	BatchesCompleted int       `yaml:"batches_completed"`
	RowsThisRun      int64     `yaml:"rows_this_run"`
	DurationSeconds  float64   `yaml:"duration_seconds"`
	EndedAt          time.Time `yaml:"ended_at"`
}

// NewState creates a new migration state
func NewState(configHash string) *State {
	sessionID := time.Now().Format("2006-01-02_15-04-05")
	errorLog := filepath.Join(config.ConfigDir, config.LogsDir, sessionID+"_errors.jsonl")

	return &State{
		ConfigHash: configHash,
		Session: SessionState{
			ID:        sessionID,
			StartedAt: time.Now(),
			ErrorLog:  errorLog,
		},
	}
}

// LoadState reads the state from the state file
func LoadState() (*State, error) {
	statePath := filepath.Join(config.ConfigDir, config.StateFile)
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No state file exists
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// SetStatePath overrides the path used by Save(). When set, Save() writes to
// this path instead of the default .pgpipe/state.yaml. Use this for per-config
// state files so that parallel CLI runs don't overwrite each other's state.
func (s *State) SetStatePath(path string) {
	s.statePath = path
}

// resolveSavePath returns the effective path to write state to.
func (s *State) resolveSavePath() string {
	if s.statePath != "" {
		return s.statePath
	}
	return filepath.Join(config.ConfigDir, config.StateFile)
}

// Save writes the state to the state file
func (s *State) Save() error {
	savePath := s.resolveSavePath()

	// Ensure the target directory exists
	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Also ensure logs dir exists (needed for ErrorLogger)
	if err := config.EnsureConfigDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(savePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// LoadStateFromPath reads state from an explicit file path. Returns (nil, nil)
// if the file does not exist.
func LoadStateFromPath(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	state.statePath = path
	return &state, nil
}

// StatePathForConfig derives a sibling state file path from a config file path.
// E.g. ./configs/individuals.yaml → ./configs/.individuals.state.yaml
// The leading dot makes it a hidden file, signalling it is machine-generated.
func StatePathForConfig(configPath string) string {
	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)

	// Strip extension, prepend dot, append .state.yaml
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, "."+name+".state.yaml")
}

// Delete removes the state file
func DeleteState() error {
	statePath := filepath.Join(config.ConfigDir, config.StateFile)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %w", err)
	}
	return nil
}

// StateExists checks if a state file exists
func StateExists() bool {
	statePath := filepath.Join(config.ConfigDir, config.StateFile)
	_, err := os.Stat(statePath)
	return err == nil
}

// IsComplete returns true if the migration has finished
func (s *State) IsComplete() bool {
	return s.Progress.LastCursor >= s.Source.MaxID
}

// ProgressPercent returns the migration progress as a percentage
func (s *State) ProgressPercent() float64 {
	if s.Source.TotalRows == 0 {
		return 0
	}
	return float64(s.Progress.ProcessedRows) / float64(s.Source.TotalRows) * 100
}

// RemainingRows returns the number of rows left to migrate
func (s *State) RemainingRows() int64 {
	return s.Source.TotalRows - s.Progress.ProcessedRows
}

// EstimatedBatchesRemaining returns the estimated number of batches left
func (s *State) EstimatedBatchesRemaining() int {
	if s.Batches.Size == 0 {
		return 0
	}
	remaining := s.RemainingRows()
	return int((remaining + int64(s.Batches.Size) - 1) / int64(s.Batches.Size))
}

// UpdateAfterBatch updates the state after a batch is processed
func (s *State) UpdateAfterBatch(lastCursor int64, processed, imported, skipped int) {
	s.Progress.LastCursor = lastCursor
	s.Progress.ProcessedRows += int64(processed)
	s.Progress.ImportedRows += int64(imported)
	s.Progress.SkippedRows += int64(skipped)
	s.Batches.Completed++
	s.LastRun.BatchesCompleted++
	s.LastRun.RowsThisRun += int64(processed)
}

// StartNewRun initializes state for a new run
func (s *State) StartNewRun(mode string, batchesRequested int) {
	s.LastRun = LastRunState{
		Mode:             mode,
		BatchesRequested: batchesRequested,
	}
}

// EndRun finalizes the run state
func (s *State) EndRun(duration time.Duration) {
	s.LastRun.DurationSeconds = duration.Seconds()
	s.LastRun.EndedAt = time.Now()
}
