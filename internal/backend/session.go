package backend

import (
	"context"

	"github.com/package-register/mocode/internal/proto"
	"github.com/package-register/mocode/internal/session"
	"github.com/package-register/mocode/internal/session/message"
)

// CreateSession creates a new session in the given workspace.
func (b *Backend) CreateSession(ctx context.Context, workspaceID, title string) (session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return session.Session{}, err
	}

	return ws.App.CreateSession(ctx, title)
}

// GetSession retrieves a session by workspace and session ID.
func (b *Backend) GetSession(ctx context.Context, workspaceID, sessionID string) (session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return session.Session{}, err
	}

	return ws.App.GetSession(ctx, sessionID)
}

// ListSessions returns all sessions in the given workspace.
func (b *Backend) ListSessions(ctx context.Context, workspaceID string) ([]session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.App.ListSessions(ctx)
}

// GetAgentSession returns session metadata with the agent's busy
// status.
func (b *Backend) GetAgentSession(ctx context.Context, workspaceID, sessionID string) (proto.AgentSession, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.AgentSession{}, err
	}

	se, err := ws.App.GetSession(ctx, sessionID)
	if err != nil {
		return proto.AgentSession{}, err
	}

	var isSessionBusy bool
	if ws.AgentCoordinator != nil {
		isSessionBusy = ws.AgentCoordinator.IsSessionBusy(sessionID)
	}

	return proto.AgentSession{
		Session: proto.Session{
			ID:    se.ID,
			Title: se.Title,
		},
		IsBusy: isSessionBusy,
	}, nil
}

// ListSessionMessages returns all messages for a session.
func (b *Backend) ListSessionMessages(ctx context.Context, workspaceID, sessionID string) ([]message.Message, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.App.Messages.List(ctx, sessionID)
}

// ListSessionHistory returns the history items for a session.
func (b *Backend) ListSessionHistory(ctx context.Context, workspaceID, sessionID string) (any, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.App.History.ListBySession(ctx, sessionID)
}

// SaveSession updates a session in the given workspace.
func (b *Backend) SaveSession(ctx context.Context, workspaceID string, sess session.Session) (session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return session.Session{}, err
	}

	return ws.App.SaveSession(ctx, sess)
}

// DeleteSession deletes a session from the given workspace.
func (b *Backend) DeleteSession(ctx context.Context, workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	return ws.App.DeleteSession(ctx, sessionID)
}

// ListUserMessages returns user-role messages for a session.
func (b *Backend) ListUserMessages(ctx context.Context, workspaceID, sessionID string) ([]message.Message, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.App.Messages.ListUserMessages(ctx, sessionID)
}

// ListAllUserMessages returns all user-role messages across sessions.
func (b *Backend) ListAllUserMessages(ctx context.Context, workspaceID string) ([]message.Message, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.App.Messages.ListAllUserMessages(ctx)
}

// UpdateMessage updates one message in the given workspace.
func (b *Backend) UpdateMessage(ctx context.Context, workspaceID string, msg message.Message) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	return ws.App.Messages.Update(ctx, msg)
}

// DeleteMessage deletes one message in the given workspace.
func (b *Backend) DeleteMessage(ctx context.Context, workspaceID, messageID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	return ws.App.Messages.Delete(ctx, messageID)
}

// BranchSession creates a new session that forks from an existing session at a snapshot point.
func (b *Backend) BranchSession(ctx context.Context, workspaceID, sessionID, snapshotID, title string) (session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return session.Session{}, err
	}

	snapSvc := session.NewSnapshotService(nil, nil)
	return snapSvc.Branch(ctx, ws.Sessions, sessionID, snapshotID, title)
}

// RevertSessionFiles restores file states to a target snapshot.
func (b *Backend) RevertSessionFiles(ctx context.Context, workspaceID, sessionID, targetSnapshotID string) ([]session.FileDiff, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	snapSvc := session.NewSnapshotService(nil, nil)
	_ = ws
	return snapSvc.RevertTo(ctx, sessionID, targetSnapshotID)
}

// ListSnapshots returns all snapshots for a session.
func (b *Backend) ListSnapshots(ctx context.Context, workspaceID, sessionID string, limit int, beforeCursor string) ([]session.SessionSnapshot, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	snapSvc := session.NewSnapshotService(nil, nil)
	_ = ws
	return snapSvc.ListSnapshots(ctx, sessionID, limit, beforeCursor)
}

// GetSnapshot returns a single snapshot by ID.
func (b *Backend) GetSnapshot(ctx context.Context, workspaceID, snapshotID string) (*session.SessionSnapshot, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	snapSvc := session.NewSnapshotService(nil, nil)
	_ = ws
	return snapSvc.GetSnapshot(ctx, snapshotID)
}

// DiffSnapshots computes file diffs between two snapshots.
func (b *Backend) DiffSnapshots(ctx context.Context, workspaceID, snapshotAID, snapshotBID string) ([]session.FileDiff, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	snapSvc := session.NewSnapshotService(nil, nil)
	_ = ws
	return snapSvc.DiffSnapshots(ctx, snapshotAID, snapshotBID)
}

// CreateSnapshot creates a new snapshot capturing current file state.
func (b *Backend) CreateSnapshot(ctx context.Context, workspaceID, sessionID, messageID, description string) (*session.SessionSnapshot, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	snapSvc := session.NewSnapshotService(nil, nil)
	_ = ws
	return snapSvc.CreateSnapshot(ctx, sessionID, messageID, description)
}
