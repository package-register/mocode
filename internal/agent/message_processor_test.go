package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTurnEndHandler_HandleTurnEnd_NoMessages(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	handler := NewTurnEndHandler(integration, "session-1", nil)

	// 没有消息时应该正常返回
	err := handler.HandleTurnEnd(context.Background())
	assert.NoError(t, err)
}

func TestTurnEndHandler_HandleTurnEnd_WithMessages(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)

	// 添加 follow-up 消息
	integration.EnqueueFollowUpMessage("session-1", "follow-up 1")
	integration.EnqueueFollowUpMessage("session-1", "follow-up 2")

	// 设置回调
	var receivedMessages []string
	callbacks := &TurnEndCallbacks{
		OnFollowUpMessage: func(ctx context.Context, sessionID string, content string) error {
			receivedMessages = append(receivedMessages, content)
			return nil
		},
	}

	handler := NewTurnEndHandler(integration, "session-1", callbacks)

	// 处理 turn 结束
	err := handler.HandleTurnEnd(context.Background())
	assert.NoError(t, err)

	// 验证消息被处理
	assert.Len(t, receivedMessages, 2)
	assert.Equal(t, "follow-up 1", receivedMessages[0])
	assert.Equal(t, "follow-up 2", receivedMessages[1])
}

func TestTurnEndHandler_HasFollowUpMessages(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	handler := NewTurnEndHandler(integration, "session-1", nil)

	// 初始状态
	assert.False(t, handler.HasFollowUpMessages())

	// 添加消息
	integration.EnqueueFollowUpMessage("session-1", "test")
	assert.True(t, handler.HasFollowUpMessages())
}

func TestTurnEndHandler_GetFollowUpCount(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	handler := NewTurnEndHandler(integration, "session-1", nil)

	// 初始状态
	assert.Equal(t, 0, handler.GetFollowUpCount())

	// 添加消息
	integration.EnqueueFollowUpMessage("session-1", "test1")
	integration.EnqueueFollowUpMessage("session-1", "test2")
	assert.Equal(t, 2, handler.GetFollowUpCount())
}

func TestTurnEndHandler_ClearFollowUpMessages(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	handler := NewTurnEndHandler(integration, "session-1", nil)

	// 添加消息
	integration.EnqueueFollowUpMessage("session-1", "test1")
	integration.EnqueueFollowUpMessage("session-1", "test2")
	assert.True(t, handler.HasFollowUpMessages())

	// 清空消息
	handler.ClearFollowUpMessages()
	assert.False(t, handler.HasFollowUpMessages())
}

func TestSteerMessageHandler_HandleSteerMessages_NoMessages(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	handler := NewSteerMessageHandler(integration, "session-1", nil)

	// 没有消息时应该返回空
	contents, err := handler.HandleSteerMessages(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, contents)
}

func TestSteerMessageHandler_HandleSteerMessages_WithMessages(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)

	// 添加 steer 消息
	integration.EnqueueSteerMessage("session-1", "steer 1")
	integration.EnqueueSteerMessage("session-1", "steer 2")

	// 设置回调
	var receivedMessages []string
	callbacks := &SteerMessageCallbacks{
		OnSteerMessage: func(ctx context.Context, sessionID string, content string) error {
			receivedMessages = append(receivedMessages, content)
			return nil
		},
	}

	handler := NewSteerMessageHandler(integration, "session-1", callbacks)

	// 处理 steer 消息
	contents, err := handler.HandleSteerMessages(context.Background())
	assert.NoError(t, err)
	assert.Len(t, contents, 2)
	assert.Equal(t, "steer 1", contents[0])
	assert.Equal(t, "steer 2", contents[1])
}

func TestSteerMessageHandler_HasSteerMessages(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	handler := NewSteerMessageHandler(integration, "session-1", nil)

	// 初始状态
	assert.False(t, handler.HasSteerMessages())

	// 添加消息
	integration.EnqueueSteerMessage("session-1", "test")
	assert.True(t, handler.HasSteerMessages())
}

func TestSteerMessageHandler_GetSteerCount(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	handler := NewSteerMessageHandler(integration, "session-1", nil)

	// 初始状态
	assert.Equal(t, 0, handler.GetSteerCount())

	// 添加消息
	integration.EnqueueSteerMessage("session-1", "test1")
	integration.EnqueueSteerMessage("session-1", "test2")
	assert.Equal(t, 2, handler.GetSteerCount())
}

func TestSteerMessageHandler_ClearSteerMessages(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	handler := NewSteerMessageHandler(integration, "session-1", nil)

	// 添加消息
	integration.EnqueueSteerMessage("session-1", "test")
	assert.True(t, handler.HasSteerMessages())

	// 清空消息
	handler.ClearSteerMessages()
	assert.False(t, handler.HasSteerMessages())
}

func TestMessageProcessor_ProcessSteerMessages(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	processor := NewMessageProcessor(integration, "session-1", nil)

	// 添加 steer 消息
	processor.EnqueueSteer("steer 1")
	processor.EnqueueSteer("steer 2")

	// 处理 steer 消息
	contents, err := processor.ProcessSteerMessages(context.Background())
	assert.NoError(t, err)
	assert.Len(t, contents, 2)
}

func TestMessageProcessor_ProcessTurnEnd(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)

	// 添加 follow-up 消息
	integration.EnqueueFollowUpMessage("session-1", "follow-up")

	// 设置回调
	var receivedMessage string
	callbacks := &MessageProcessorCallbacks{
		OnFollowUpMessage: func(ctx context.Context, sessionID string, content string) error {
			receivedMessage = content
			return nil
		},
	}

	processor := NewMessageProcessor(integration, "session-1", callbacks)

	// 处理 turn 结束
	err := processor.ProcessTurnEnd(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "follow-up", receivedMessage)
}

func TestMessageProcessor_HasMessages(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	processor := NewMessageProcessor(integration, "session-1", nil)

	// 初始状态
	assert.False(t, processor.HasMessages())

	// 添加 steer 消息
	processor.EnqueueSteer("test")
	assert.True(t, processor.HasMessages())

	// 清空
	processor.Clear()
	assert.False(t, processor.HasMessages())

	// 添加 follow-up 消息
	processor.EnqueueFollowUp("test")
	assert.True(t, processor.HasMessages())
}

func TestMessageProcessor_GetStatus(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	processor := NewMessageProcessor(integration, "session-1", nil)

	// 添加消息
	processor.EnqueueSteer("steer")
	processor.EnqueueFollowUp("follow")

	// 获取状态
	status := processor.GetStatus()
	assert.Equal(t, 1, status.SteerDepth)
	assert.Equal(t, 1, status.FollowupDepth)
}

func TestMessageProcessor_Clear(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	processor := NewMessageProcessor(integration, "session-1", nil)

	// 添加消息
	processor.EnqueueSteer("steer")
	processor.EnqueueFollowUp("follow")
	assert.True(t, processor.HasMessages())

	// 清空
	processor.Clear()
	assert.False(t, processor.HasMessages())
}

func TestMessageProcessor_EnqueueSteer(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	processor := NewMessageProcessor(integration, "session-1", nil)

	// 添加 steer 消息
	result := processor.EnqueueSteer("test")
	assert.True(t, result)
	assert.True(t, processor.HasSteerMessages())
}

func TestMessageProcessor_EnqueueFollowUp(t *testing.T) {
	integration := NewMessageQueueIntegration(nil)
	processor := NewMessageProcessor(integration, "session-1", nil)

	// 添加 follow-up 消息
	result := processor.EnqueueFollowUp("test")
	assert.True(t, result)
	assert.True(t, processor.HasFollowUpMessages())
}
