package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/luanlima/gaal-lib/pkg/types"
)

var unsafeFileChars = regexp.MustCompile(`[^a-zA-Z0-9\-_.]`)

// FileStore is a durable Store implementation backed by JSON files on the local
// filesystem. Each session is persisted as a single file named after the
// sanitized sessionID inside the configured directory.
//
// FileStore is safe for concurrent use within a single process. Cross-process
// locking is not provided in this version.
//
// Characters in sessionID that are not alphanumeric, dash, underscore or dot
// are replaced with underscore for the filename. This means two distinct
// sessionIDs that differ only by such characters will collide.
type FileStore struct {
	dir string
	mu  sync.RWMutex
}

// NewFileStore creates a FileStore rooted at dir. The directory is created if it
// does not exist. Returns an error when dir is empty or cannot be created.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, errors.New("memory: file store directory must not be empty")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("memory: create directory %q: %w", dir, err)
	}

	return &FileStore{dir: dir}, nil
}

// MustNewFileStore is like NewFileStore but panics on error.
// Intended for examples and demos.
func MustNewFileStore(dir string) *FileStore {
	s, err := NewFileStore(dir)
	if err != nil {
		panic(err)
	}
	return s
}

type filePayload struct {
	Messages []types.Message `json:"messages"`
	Records  []Record        `json:"records"`
	Metadata types.Metadata  `json:"metadata"`
}

// Load reads the persisted conversational snapshot for sessionID from disk.
// A missing or empty file returns an empty Snapshot without error.
func (s *FileStore) Load(ctx context.Context, sessionID string) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.sessionPath(sessionID)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Snapshot{}, nil
		}
		return Snapshot{}, fmt.Errorf("memory: read %q: %w", path, err)
	}

	if len(data) == 0 {
		return Snapshot{}, nil
	}

	var payload filePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return Snapshot{}, fmt.Errorf("memory: decode %q: %w", path, err)
	}

	return Snapshot{
		Messages: types.CloneMessages(payload.Messages),
		Records:  cloneRecords(payload.Records),
		Metadata: types.CloneMetadata(payload.Metadata),
	}, nil
}

// Save persists the conversational state for sessionID to disk using atomic
// write-rename. The payload is serialized as indented JSON for human
// readability. Delta.Response is not stored separately because the response
// message is already included in Delta.Messages by the runtime.
func (s *FileStore) Save(ctx context.Context, sessionID string, delta Delta) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	payload := filePayload{
		Messages: types.CloneMessages(delta.Messages),
		Records:  cloneRecords(delta.Records),
		Metadata: types.CloneMetadata(delta.Metadata),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("memory: encode session %q: %w", sessionID, err)
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	target := s.sessionPath(sessionID)

	tmp, err := os.CreateTemp(s.dir, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("memory: create temp in %q: %w", s.dir, err)
	}
	tmpPath := tmp.Name()

	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, writeErr := tmp.Write(data); writeErr != nil {
		return fmt.Errorf("memory: write %q: %w", tmpPath, writeErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		return fmt.Errorf("memory: close %q: %w", tmpPath, closeErr)
	}
	if renameErr := os.Rename(tmpPath, target); renameErr != nil {
		return fmt.Errorf("memory: rename %q -> %q: %w", tmpPath, target, renameErr)
	}

	committed = true
	return nil
}

func (s *FileStore) sessionPath(sessionID string) string {
	return filepath.Join(s.dir, sanitizeSessionID(sessionID)+".json")
}

func sanitizeSessionID(id string) string {
	return unsafeFileChars.ReplaceAllString(id, "_")
}
