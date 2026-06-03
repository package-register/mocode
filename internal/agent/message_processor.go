package agent

import (
	"context"
	"log/slog"
)

// TurnEndHandler turn 结束处理器
// 处理 follow-up 消息，在 turn 结束时调用
type TurnEndHandler struct {
	integration *MessageQueueIntegration
	sessionID   string
	callbacks   *TurnEndCallbacks
}

// TurnEndCallbacks turn 结束回调
type TurnEndCallbacks struct {
	// OnFollowUpMessage 收到 follow-up 消息时的回调
	OnFollowUpMessage func(ctx context.Context, sessionID string, content string) error
	// OnTurnComplete turn 完成时的回调
	OnTurnComplete func(ctx context.Context, sessionID string) error
}

// NewTurnEndHandler 创建 turn 结束处理器
func NewTurnEndHandler(integration *MessageQueueIntegration, sessionID string, callbacks *TurnEndCallbacks) *TurnEndHandler {
	return &TurnEndHandler{
		integration: integration,
		sessionID:   sessionID,
		callbacks:   callbacks,
	}
}

// HandleTurnEnd 处理 turn 结束
// 检查是否有 follow-up 消息，如果有则处理
func (h *TurnEndHandler) HandleTurnEnd(ctx context.Context) error {
	// 检查是否有 follow-up 消息
	if !h.integration.HasFollowUpMessages() {
		slog.DebugContext(ctx, "No follow-up messages to process", "session_id", h.sessionID)
		return nil
	}

	// 处理 follow-up 消息
	for {
		msg, ok := h.integration.GetFollowUpMessage()
		if !ok {
			break
		}

		slog.InfoContext(ctx, "Processing follow-up message",
			"session_id", h.sessionID,
			"content", msg.Content,
		)

		// 调用回调
		if h.callbacks != nil && h.callbacks.OnFollowUpMessage != nil {
			if err := h.callbacks.OnFollowUpMessage(ctx, h.sessionID, msg.Content); err != nil {
				slog.ErrorContext(ctx, "Failed to process follow-up message",
					"session_id", h.sessionID,
					"error", err,
				)
				return err
			}
		}
	}

	// 通知 turn 完成
	if h.callbacks != nil && h.callbacks.OnTurnComplete != nil {
		if err := h.callbacks.OnTurnComplete(ctx, h.sessionID); err != nil {
			slog.ErrorContext(ctx, "Failed to notify turn complete",
				"session_id", h.sessionID,
				"error", err,
			)
			return err
		}
	}

	return nil
}

// HasFollowUpMessages 检查是否有 follow-up 消息
func (h *TurnEndHandler) HasFollowUpMessages() bool {
	return h.integration.HasFollowUpMessages()
}

// GetFollowUpCount 获取 follow-up 消息数量
func (h *TurnEndHandler) GetFollowUpCount() int {
	status := h.integration.GetStatus()
	return status.FollowupDepth
}

// ClearFollowUpMessages 清空 follow-up 消息
func (h *TurnEndHandler) ClearFollowUpMessages() {
	// 逐个取出并丢弃
	for {
		_, ok := h.integration.GetFollowUpMessage()
		if !ok {
			break
		}
	}
}

// SteerMessageHandler steer 消息处理器
// 处理紧急消息，在 PrepareStep 中调用
type SteerMessageHandler struct {
	integration *MessageQueueIntegration
	sessionID   string
	callbacks   *SteerMessageCallbacks
}

// SteerMessageCallbacks steer 消息回调
type SteerMessageCallbacks struct {
	// OnSteerMessage 收到 steer 消息时的回调
	OnSteerMessage func(ctx context.Context, sessionID string, content string) error
}

// NewSteerMessageHandler 创建 steer 消息处理器
func NewSteerMessageHandler(integration *MessageQueueIntegration, sessionID string, callbacks *SteerMessageCallbacks) *SteerMessageHandler {
	return &SteerMessageHandler{
		integration: integration,
		sessionID:   sessionID,
		callbacks:   callbacks,
	}
}

// HandleSteerMessages 处理 steer 消息
// 在 PrepareStep 中调用，注入紧急消息
func (h *SteerMessageHandler) HandleSteerMessages(ctx context.Context) ([]string, error) {
	// 检查是否有 steer 消息
	if !h.integration.HasSteerMessages() {
		return nil, nil
	}

	// 处理 steer 消息
	var contents []string
	for {
		msgs := h.integration.GetSteerMessages()
		if len(msgs) == 0 {
			break
		}

		for _, msg := range msgs {
			slog.InfoContext(ctx, "Processing steer message",
				"session_id", h.sessionID,
				"content", msg.Content,
			)

			contents = append(contents, msg.Content)

			// 调用回调
			if h.callbacks != nil && h.callbacks.OnSteerMessage != nil {
				if err := h.callbacks.OnSteerMessage(ctx, h.sessionID, msg.Content); err != nil {
					slog.ErrorContext(ctx, "Failed to process steer message",
						"session_id", h.sessionID,
						"error", err,
					)
					return nil, err
				}
			}
		}
	}

	return contents, nil
}

// HasSteerMessages 检查是否有 steer 消息
func (h *SteerMessageHandler) HasSteerMessages() bool {
	return h.integration.HasSteerMessages()
}

// GetSteerCount 获取 steer 消息数量
func (h *SteerMessageHandler) GetSteerCount() int {
	status := h.integration.GetStatus()
	return status.SteerDepth
}

// ClearSteerMessages 清空 steer 消息
func (h *SteerMessageHandler) ClearSteerMessages() {
	h.integration.Clear()
}

// MessageProcessor 消息处理器
// 统一处理 steer 和 follow-up 消息
type MessageProcessor struct {
	integration    *MessageQueueIntegration
	sessionID      string
	steerHandler   *SteerMessageHandler
	turnEndHandler *TurnEndHandler
}

// NewMessageProcessor 创建消息处理器
func NewMessageProcessor(integration *MessageQueueIntegration, sessionID string, callbacks *MessageProcessorCallbacks) *MessageProcessor {
	steerCallbacks := &SteerMessageCallbacks{}
	turnEndCallbacks := &TurnEndCallbacks{}

	if callbacks != nil {
		steerCallbacks.OnSteerMessage = callbacks.OnSteerMessage
		turnEndCallbacks.OnFollowUpMessage = callbacks.OnFollowUpMessage
		turnEndCallbacks.OnTurnComplete = callbacks.OnTurnComplete
	}

	return &MessageProcessor{
		integration:    integration,
		sessionID:      sessionID,
		steerHandler:   NewSteerMessageHandler(integration, sessionID, steerCallbacks),
		turnEndHandler: NewTurnEndHandler(integration, sessionID, turnEndCallbacks),
	}
}

// MessageProcessorCallbacks 消息处理器回调
type MessageProcessorCallbacks struct {
	// OnSteerMessage 收到 steer 消息时的回调
	OnSteerMessage func(ctx context.Context, sessionID string, content string) error
	// OnFollowUpMessage 收到 follow-up 消息时的回调
	OnFollowUpMessage func(ctx context.Context, sessionID string, content string) error
	// OnTurnComplete turn 完成时的回调
	OnTurnComplete func(ctx context.Context, sessionID string) error
}

// ProcessSteerMessages 处理 steer 消息
func (p *MessageProcessor) ProcessSteerMessages(ctx context.Context) ([]string, error) {
	return p.steerHandler.HandleSteerMessages(ctx)
}

// ProcessTurnEnd 处理 turn 结束
func (p *MessageProcessor) ProcessTurnEnd(ctx context.Context) error {
	return p.turnEndHandler.HandleTurnEnd(ctx)
}

// HasMessages 检查是否有待处理消息
func (p *MessageProcessor) HasMessages() bool {
	return p.integration.HasMessages()
}

// HasSteerMessages 检查是否有 steer 消息
func (p *MessageProcessor) HasSteerMessages() bool {
	return p.integration.HasSteerMessages()
}

// HasFollowUpMessages 检查是否有 follow-up 消息
func (p *MessageProcessor) HasFollowUpMessages() bool {
	return p.integration.HasFollowUpMessages()
}

// GetStatus 获取队列状态
func (p *MessageProcessor) GetStatus() QueueStatus {
	return p.integration.GetStatus()
}

// Clear 清空所有队列
func (p *MessageProcessor) Clear() {
	p.integration.Clear()
}

// EnqueueSteer 添加 steer 消息
func (p *MessageProcessor) EnqueueSteer(content string) bool {
	return p.integration.EnqueueSteerMessage(p.sessionID, content)
}

// EnqueueFollowUp 添加 follow-up 消息
func (p *MessageProcessor) EnqueueFollowUp(content string) bool {
	return p.integration.EnqueueFollowUpMessage(p.sessionID, content)
}
