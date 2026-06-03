package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/package-register/mocode/internal/config"
	"github.com/package-register/mocode/internal/session/message"
	"github.com/package-register/mocode/internal/store"
)

func tempSvcDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "mocode-svc-test-"+t.Name())
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestRollbackService_TruncateToMessage_NoOp(t *testing.T) {
	s := newStoreForTest(t)
	defer s.Close()

	sess, err := s.Sessions().Create(t.Context(), "Test")
	if err != nil {
		t.Fatal(err)
	}

	var msgIDs []string
	for i := 0; i < 3; i++ {
		msg, err := s.Messages().Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		msgIDs = append(msgIDs, msg.ID)
	}

	svc := NewRollbackService(s)

	// Truncate to last message — should be a no-op
	result, err := svc.TruncateToMessage(t.Context(), sess.ID, msgIDs[2], "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	if result.MessagesDeleted != 0 {
		t.Errorf("expected 0 deleted, got %d", result.MessagesDeleted)
	}

	msgs, err := s.Messages().List(t.Context(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages, got %d", len(msgs))
	}
}

func TestRollbackService_TruncateToMessage_Middle(t *testing.T) {
	s := newStoreForTest(t)
	defer s.Close()

	sess, err := s.Sessions().Create(t.Context(), "Test")
	if err != nil {
		t.Fatal(err)
	}

	var msgIDs []string
	for i := 0; i < 5; i++ {
		msg, err := s.Messages().Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		msgIDs = append(msgIDs, msg.ID)
		time.Sleep(1 * time.Millisecond)
	}

	svc := NewRollbackService(s)
	result, err := svc.TruncateToMessage(t.Context(), sess.ID, msgIDs[2], "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	if result.MessagesDeleted != 2 {
		t.Errorf("expected 2 deleted, got %d", result.MessagesDeleted)
	}
	if result.MessageIndex != 3 {
		t.Errorf("expected index 3, got %d", result.MessageIndex)
	}

	msgs, err := s.Messages().List(t.Context(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	for i, msg := range msgs {
		if msg.ID != msgIDs[i] {
			t.Errorf("message %d: expected ID %s, got %s", i, msgIDs[i], msg.ID)
		}
	}
}

func TestRollbackService_TruncateToMessage_First(t *testing.T) {
	s := newStoreForTest(t)
	defer s.Close()

	sess, err := s.Sessions().Create(t.Context(), "Test")
	if err != nil {
		t.Fatal(err)
	}

	var msgIDs []string
	for i := 0; i < 4; i++ {
		msg, err := s.Messages().Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		msgIDs = append(msgIDs, msg.ID)
		time.Sleep(1 * time.Millisecond)
	}

	svc := NewRollbackService(s)
	result, err := svc.TruncateToMessage(t.Context(), sess.ID, msgIDs[0], "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	if result.MessagesDeleted != 3 {
		t.Errorf("expected 3 deleted, got %d", result.MessagesDeleted)
	}

	msgs, err := s.Messages().List(t.Context(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestRollbackService_TruncateToMessage_NotFound(t *testing.T) {
	s := newStoreForTest(t)
	defer s.Close()

	sess, err := s.Sessions().Create(t.Context(), "Test")
	if err != nil {
		t.Fatal(err)
	}

	svc := NewRollbackService(s)
	_, err = svc.TruncateToMessage(t.Context(), sess.ID, "nonexistent-id", "/tmp/test")
	if err == nil {
		t.Fatal("expected error for nonexistent message")
	}
}

// newStoreForTest creates a Store for testing using store.New with a minimal config.
func newStoreForTest(t *testing.T) *store.Store {
	t.Helper()
	dir := tempSvcDir(t)

	// Prepare minimal config with custom DataDir
	cfg := config.NewTestStore(&config.Config{})
	s, err := store.New(dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
