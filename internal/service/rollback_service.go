package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/package-register/mocode/internal/history"
	"github.com/package-register/mocode/internal/infra/home"
	"github.com/package-register/mocode/internal/session"
	"github.com/package-register/mocode/internal/store"
)

// RollbackService handles context rollback operations,
// including message truncation, file restoration, and git snapshots.
type RollbackService struct {
	store *store.Store
}

// NewRollbackService creates a new RollbackService.
func NewRollbackService(s *store.Store) *RollbackService {
	return &RollbackService{store: s}
}

// RollbackResult contains the result of a rollback operation.
type RollbackResult struct {
	// MessageIndex is the 1-based index of the target message.
	MessageIndex int
	// MessagesDeleted is the number of messages removed.
	MessagesDeleted int
	// FilesRestored is the number of files restored to their earlier version.
	FilesRestored int
	// FilesRemoved is the number of files that did not exist at the target time.
	FilesRemoved int
	// SnapshotPath is the path to the git snapshot directory.
	SnapshotPath string
}

// TruncateToMessage truncates the session to the specified message,
// removing all messages after it and rolling back modified files.
//
// Flow:
//  1. Find the target message and count how many messages will be removed
//  2. If there are file histories, create a pre-rollback git snapshot
//  3. Restore files to their state at the target message's timestamp
//  4. Create a post-rollback git snapshot
//  5. Truncate messages after the target
func (rs *RollbackService) TruncateToMessage(ctx context.Context, sessionID string, messageID string, workingDir string) (*RollbackResult, error) {
	// Get all messages for the session
	messages, err := rs.store.Messages().List(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	// Find the target message index
	targetIdx := -1
	for i, msg := range messages {
		if msg.ID == messageID {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}

	// No messages to delete — nothing to do
	messagesToDelete := len(messages) - targetIdx - 1
	if messagesToDelete <= 0 {
		return &RollbackResult{
			MessageIndex:    targetIdx + 1,
			MessagesDeleted: 0,
		}, nil
	}

	// Get file history for this session
	files, err := rs.store.Files().ListBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list file history: %w", err)
	}

	// Get the session for snapshot directory resolution
	sess, err := rs.store.Sessions().Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	result := &RollbackResult{
		MessageIndex:    targetIdx + 1,
		MessagesDeleted: messagesToDelete,
	}

	// Handle file rollback if there's file history
	if len(files) > 0 {
		snapshotDir, err := rollbackSnapshotDir(sess, workingDir)
		if err != nil {
			return nil, fmt.Errorf("snapshot dir: %w", err)
		}

		// Pre-rollback snapshot (capture current state before changes)
		preState := currentFileState(files, workingDir)
		if err := commitSnapshot(snapshotDir, workingDir, preState, fmt.Sprintf("pre-rollback to #%d %s", targetIdx+1, shortMsgID(messageID))); err != nil {
			return nil, fmt.Errorf("pre-rollback snapshot: %w", err)
		}

		// Restore files to target message's timestamp
		targetTime := messages[targetIdx].UpdatedAt
		restored, removed, err := restoreFilesToTime(files, workingDir, targetTime)
		if err != nil {
			return nil, fmt.Errorf("restore files: %w", err)
		}
		result.FilesRestored = restored
		result.FilesRemoved = removed
		result.SnapshotPath = snapshotDir

		// Post-rollback snapshot (capture restored state)
		postState := currentFileState(files, workingDir)
		if err := commitSnapshot(snapshotDir, workingDir, postState, fmt.Sprintf("post-rollback to #%d %s", targetIdx+1, shortMsgID(messageID))); err != nil {
			return nil, fmt.Errorf("post-rollback snapshot: %w", err)
		}
	}

	// Truncate messages after the target
	if err := rs.store.Messages().DeleteAfter(ctx, sessionID, targetIdx); err != nil {
		return nil, fmt.Errorf("truncate messages: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// File restoration
// ---------------------------------------------------------------------------

// restoreFilesToTime restores working-directory files to their state at
// the given cutoff timestamp, using the file history records.
func restoreFilesToTime(files []history.File, workingDir string, cutoff int64) (restored, removed int, err error) {
	byPath := map[string][]history.File{}
	for _, f := range files {
		byPath[f.Path] = append(byPath[f.Path], f)
	}

	for path, versions := range byPath {
		// Sort versions by (CreatedAt, Version) ascending
		sortHistoryFiles(versions)

		// Find the latest version at or before cutoff
		var target *history.File
		for i := range versions {
			if versions[i].CreatedAt <= cutoff {
				target = &versions[i]
			}
		}

		absPath := resolveAbsPath(workingDir, path)
		if target == nil {
			// File did not exist at cutoff — remove it
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				return restored, removed, fmt.Errorf("remove %s: %w", path, err)
			}
			removed++
			continue
		}

		// Restore file content
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return restored, removed, fmt.Errorf("create parent for %s: %w", path, err)
		}
		if err := os.WriteFile(absPath, []byte(target.Content), 0o644); err != nil {
			return restored, removed, fmt.Errorf("restore %s: %w", path, err)
		}
		restored++
	}
	return restored, removed, nil
}

// sortHistoryFiles sorts a slice of history.File by (CreatedAt, Version) ascending.
func sortHistoryFiles(files []history.File) {
	for i := 1; i < len(files); i++ {
		key := files[i]
		j := i - 1
		for j >= 0 && (files[j].CreatedAt > key.CreatedAt ||
			(files[j].CreatedAt == key.CreatedAt && files[j].Version > key.Version)) {
			files[j+1] = files[j]
			j--
		}
		files[j+1] = key
	}
}

// currentFileState reads the current on-disk content of all tracked files.
func currentFileState(files []history.File, workingDir string) map[string]string {
	paths := map[string]struct{}{}
	for _, f := range files {
		paths[f.Path] = struct{}{}
	}
	state := make(map[string]string, len(paths))
	for path := range paths {
		content, err := os.ReadFile(resolveAbsPath(workingDir, path))
		if err != nil {
			continue
		}
		state[path] = string(content)
	}
	return state
}

// ---------------------------------------------------------------------------
// Git snapshot helpers
// ---------------------------------------------------------------------------

// rollbackSnapshotDir returns the directory where rollback git snapshots are stored.
func rollbackSnapshotDir(sess session.Session, workingDir string) (string, error) {
	project := filepath.Base(workingDir)
	if project == "." || project == string(filepath.Separator) || project == "" {
		project = "project"
	}
	root := filepath.Join(home.Dir(), ".mocode", "screens-shops")
	sessionDir := filepath.Base(session.StoreDir(root, sess))
	return filepath.Join(root, sessionDir, safeFileName(project)), nil
}

// commitSnapshot creates a git commit in the snapshot repo capturing the given file state.
func commitSnapshot(repoDir, workingDir string, state map[string]string, msg string) error {
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return err
	}

	repo, err := git.PlainOpen(repoDir)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		repo, err = git.PlainInit(repoDir, false)
	}
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Clear worktree (keep .git)
	if err := clearSnapshotWorktree(repoDir); err != nil {
		return err
	}

	// Write current state into the snapshot worktree
	for path, content := range state {
		rel, ok := relPath(workingDir, path)
		if !ok {
			continue
		}
		dst := filepath.Join(repoDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			return err
		}
		if _, err := wt.Add(rel); err != nil {
			return err
		}
	}

	// Commit
	_, err = wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "mocode", Email: "mocode@local", When: time.Now()},
		All:    true,
	})
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "clean") {
		return err
	}
	return nil
}

// clearSnapshotWorktree removes all files except .git from the snapshot repo.
func clearSnapshotWorktree(repoDir string) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(repoDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Path utilities
// ---------------------------------------------------------------------------

// resolveAbsPath returns the absolute path for a tracked file.
func resolveAbsPath(workingDir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(workingDir, path)
}

// relPath computes a relative path from workingDir suitable for the snapshot repo.
func relPath(workingDir, path string) (string, bool) {
	abs := resolveAbsPath(workingDir, path)
	rel, err := filepath.Rel(workingDir, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// safeFileName replaces unsafe characters for file system paths.
func safeFileName(name string) string {
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, name)
	name = strings.Trim(name, "-._")
	if name == "" {
		return "project"
	}
	return name
}

// shortMsgID returns a short prefix of a message ID for display.
func shortMsgID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}


