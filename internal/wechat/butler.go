// butler.go - LLM-powered butler agent with persistent session and context.
package wechat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ButlerContext carries the dependencies for the LLM butler.
type ButlerContext struct {
	Channel   *Channel
	Workspace ButlerWorkspace
}

// ButlerWorkspace is the subset of workspace.Workspace the butler needs.
type ButlerWorkspace interface {
	CreateSession(ctx context.Context, title string) (string, error)
	ListSessions(ctx context.Context) ([]SessionInfo, error)
	DeleteSession(ctx context.Context, sessionID string) error
	AgentRun(ctx context.Context, sessionID, prompt string) error
	ListMessages(ctx context.Context, sessionID string) ([]MsgInfo, error)
}

// SessionInfo is a lightweight session summary.
type SessionInfo struct {
	ID        string
	Title     string
	CreatedAt string
}

// MsgInfo is a lightweight message summary.
type MsgInfo struct {
	Role    string
	Content string
}

// llmButlerHandler is the LLM-powered butler.
type llmButlerHandler struct {
	ctx         *ButlerContext
	mu          sync.Mutex
	sessID      string // butler's own agent session ID
	initialized bool   // true after first system prompt sent
}

// newButlerHandler creates the LLM butler handler.
func newButlerHandler(butlerCtx *ButlerContext) *llmButlerHandler {
	return &llmButlerHandler{ctx: butlerCtx}
}

const butlerAgentTimeout = 5 * time.Minute

// Handle processes a user message through the LLM butler.
func (h *llmButlerHandler) Handle(pollCtx context.Context, userID, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	// Slash commands handled fast (no LLM).
	if strings.HasPrefix(text, "/") {
		return h.handleButlerSlash(pollCtx, userID, text)
	}

	// Ensure butler session exists and is primed.
	if err := h.ensureSession(pollCtx, userID, text); err != nil {
		slog.Error("Butler session init failed", "error", err)
		return "抱歉，系统正在初始化，请稍后再试。"
	}

	// Send typing indicator while butler thinks.
	stopTyping := h.ctx.Channel.StartTyping(pollCtx, userID)
	defer stopTyping()

	// G5: Add timeout.
	ctx, cancel := context.WithTimeout(pollCtx, butlerAgentTimeout)
	defer cancel()

	// G4+G8: Discover and include existing sessions in the prompt.
	userPrompt := h.buildUserPrompt(ctx, userID, text)

	// Run the butler agent. Blocks until LLM finishes.
	if err := h.ctx.Workspace.AgentRun(ctx, h.sessID, userPrompt); err != nil {
		slog.Error("Butler AgentRun failed", "error", err)
		return "抱歉，处理出错了。" + err.Error()
	}

	// Extract the last assistant reply.
	msgs, err := h.ctx.Workspace.ListMessages(ctx, h.sessID)
	if err != nil || len(msgs) == 0 {
		return "处理完成。"
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return msgs[i].Content
		}
	}
	return "处理完成。"
}

// ensureSession initializes the butler session. On first call, creates the
// session and injects the system prompt. On subsequent calls, no-op.
func (h *llmButlerHandler) ensureSession(ctx context.Context, userID, _ string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.initialized {
		return nil
	}
	if h.ctx.Workspace == nil {
		return fmt.Errorf("workspace not available")
	}

	// G1: Create session once.
	sid, err := h.ctx.Workspace.CreateSession(ctx, "Butler")
	if err != nil {
		return err
	}
	h.sessID = sid
	slog.Info("Butler session created", "sessionID", sid)

	// G1: Inject system prompt as first message (only once).
	prompt := "系统指令:\n\n" + ButlerSystemPrompt + "\n\n请确认你已理解以上指令，回复OK。"
	if err := h.ctx.Workspace.AgentRun(ctx, h.sessID, prompt); err != nil {
		return fmt.Errorf("system prompt injection: %w", err)
	}
	h.initialized = true
	slog.Info("Butler system prompt injected", "sessionID", sid)
	return nil
}

// buildUserPrompt builds the prompt with session context.
func (h *llmButlerHandler) buildUserPrompt(ctx context.Context, userID, text string) string {
	// G4+G8: Include session list so butler knows available sessions.
	var b strings.Builder

	// Discover sessions.
	sessions, err := h.ctx.Workspace.ListSessions(ctx)
	if err == nil && len(sessions) > 0 {
		// Filter out butler's own session.
		h.mu.Lock()
		butlerSID := h.sessID
		h.mu.Unlock()

		b.WriteString("当前可用会话:\n")
		for _, s := range sessions {
			if s.ID == butlerSID {
				continue
			}
			b.WriteString(fmt.Sprintf("- ID: %s  标题: %s  (%s)\n", s.ID, s.Title, s.CreatedAt))
		}
		b.WriteString("\n")
	}

	// User's session binding.
	sessID, ok := h.ctx.Channel.GetSession(userID)
	if ok && sessID != "" {
		b.WriteString(fmt.Sprintf("用户 %s 的绑定会话 ID 是: %s\n\n", userID, sessID))
	}

	b.WriteString(fmt.Sprintf("用户 %s 说: %s", userID, text))
	return b.String()
}

// handleButlerSlash handles slash commands at the butler level.
func (h *llmButlerHandler) handleButlerSlash(ctx context.Context, userID, text string) string {
	parts := strings.SplitN(text, " ", 2)
	cmd := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	switch cmd {
	case "/new":
		return h.createSession(ctx, userID, args)
	case "/switch":
		if args == "" {
			return "用法: /switch <会话ID>"
		}
		h.ctx.Channel.mu.Lock()
		h.ctx.Channel.activeSession = args
		h.ctx.Channel.mu.Unlock()
		return fmt.Sprintf("✅ 已切换到: %s", args)
	case "/delete":
		if args == "" {
			return "用法: /delete <会话ID>"
		}
		return h.deleteSession(ctx, args)
	default:
		return fmt.Sprintf("未知命令: %s", cmd)
	}
}

func (h *llmButlerHandler) createSession(ctx context.Context, userID, name string) string {
	if h.ctx.Workspace == nil {
		return "❌ 系统未就绪"
	}
	title := name
	if title == "" {
		title = fmt.Sprintf("WeChat: %s", userID)
	}
	sessionID, err := h.ctx.Workspace.CreateSession(ctx, title)
	if err != nil {
		return fmt.Sprintf("❌ 创建失败: %v", err)
	}
	h.ctx.Channel.SetSession(userID, sessionID)
	h.ctx.Channel.mu.Lock()
	h.ctx.Channel.activeSession = userID
	h.ctx.Channel.mu.Unlock()
	return fmt.Sprintf("✅ 会话已创建\n  ID: %s\n  标题: %s", sessionID, title)
}

func (h *llmButlerHandler) deleteSession(ctx context.Context, sessionID string) string {
	if h.ctx.Workspace == nil {
		return "❌ 系统未就绪"
	}
	if err := h.ctx.Workspace.DeleteSession(ctx, sessionID); err != nil {
		return fmt.Sprintf("❌ 删除失败: %v", err)
	}
	return "✅ 会话已删除"
}
