package agent

import (
	"context"
	"log/slog"

	"charm.land/fantasy"
)

// MessageQueueIntegration 消息队列集成
// 将 MessageQueueManager 集成到现有 agent 系统中
type MessageQueueIntegration struct {
	manager *MessageQueueManager
	agent   SessionAgent
}

// NewMessageQueueIntegration 创建消息队列集成
func NewMessageQueueIntegration(agent SessionAgent) *MessageQueueIntegration {
	return &MessageQueueIntegration{
		manager: NewMessageQueueManager(),
		agent:   agent,
	}
}

// EnqueueSteerMessage 添加紧急消息
func (i *MessageQueueIntegration) EnqueueSteerMessage(sessionID, content string) bool {
	return i.manager.EnqueueSteer(QueuedMessage{
		Content:  content,
		Priority: PrioritySteer,
	})
}

// EnqueueFollowUpMessage 添加普通消息
func (i *MessageQueueIntegration) EnqueueFollowUpMessage(sessionID, content string) bool {
	return i.manager.EnqueueFollowUp(QueuedMessage{
		Content:  content,
		Priority: PriorityFollowUp,
	})
}

// GetSteerMessages 获取所有紧急消息
func (i *MessageQueueIntegration) GetSteerMessages() []QueuedMessage {
	return i.manager.DrainSteer()
}

// GetFollowUpMessage 获取一个普通消息
func (i *MessageQueueIntegration) GetFollowUpMessage() (QueuedMessage, bool) {
	return i.manager.DequeueFollowUp()
}

// HasMessages 检查是否有待处理消息
func (i *MessageQueueIntegration) HasMessages() bool {
	return i.manager.HasMessages()
}

// HasSteerMessages 检查是否有紧急消息
func (i *MessageQueueIntegration) HasSteerMessages() bool {
	return i.manager.HasSteerMessages()
}

// HasFollowUpMessages 检查是否有普通消息
func (i *MessageQueueIntegration) HasFollowUpMessages() bool {
	return i.manager.HasFollowUpMessages()
}

// GetStatus 获取队列状态
func (i *MessageQueueIntegration) GetStatus() QueueStatus {
	return i.manager.Status()
}

// Clear 清空所有队列
func (i *MessageQueueIntegration) Clear() {
	i.manager.Clear()
}

// ProcessSteerMessages 处理紧急消息
// 在 PrepareStep 中调用，只注入紧急消息
func (i *MessageQueueIntegration) ProcessSteerMessages(ctx context.Context, sessionID string) []string {
	messages := i.GetSteerMessages()
	if len(messages) == 0 {
		return nil
	}

	var contents []string
	for _, msg := range messages {
		contents = append(contents, msg.Content)
		slog.DebugContext(ctx, "Processing steer message",
			"session_id", sessionID,
			"content", msg.Content,
		)
	}

	return contents
}

// ProcessFollowUpMessages 处理普通消息
// 在 turn 结束时调用，开始新的 turn
func (i *MessageQueueIntegration) ProcessFollowUpMessages(ctx context.Context, sessionID string) *string {
	msg, ok := i.GetFollowUpMessage()
	if !ok {
		return nil
	}

	slog.DebugContext(ctx, "Processing follow-up message",
		"session_id", sessionID,
		"content", msg.Content,
	)

	return &msg.Content
}

// WrapPrepareStep 包装 PrepareStep 函数
// 只处理紧急消息，不处理普通消息
func (i *MessageQueueIntegration) WrapPrepareStep(
	originalPrepareStep func(context.Context, fantasy.PrepareStepFunctionOptions) (context.Context, fantasy.PrepareStepResult, error),
	sessionID string,
) func(context.Context, fantasy.PrepareStepFunctionOptions) (context.Context, fantasy.PrepareStepResult, error) {
	return func(ctx context.Context, options fantasy.PrepareStepFunctionOptions) (context.Context, fantasy.PrepareStepResult, error) {
		// 调用原始 PrepareStep
		ctx, prepared, err := originalPrepareStep(ctx, options)
		if err != nil {
			return ctx, prepared, err
		}

		// 只注入紧急消息
		steerContents := i.ProcessSteerMessages(ctx, sessionID)
		for _, content := range steerContents {
			// 将紧急消息添加到 prepared.Messages
			// 这里需要根据实际的消息格式进行转换
			slog.DebugContext(ctx, "Injecting steer message", "content", content)
		}

		return ctx, prepared, nil
	}
}

// OnTurnEnd turn 结束时的回调
// 处理普通消息，开始新的 turn
func (i *MessageQueueIntegration) OnTurnEnd(ctx context.Context, sessionID string) *string {
	return i.ProcessFollowUpMessages(ctx, sessionID)
}
