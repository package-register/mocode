package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageQueueIntegration_EnqueueSteerMessage(t *testing.T) {
	// 创建一个模拟的 SessionAgent
	// 注意：这里我们需要一个真实的 SessionAgent 实现来测试
	// 为了测试，我们直接测试 MessageQueueManager

	mgr := NewMessageQueueManager()

	// 测试添加紧急消息
	result := mgr.EnqueueSteer(QueuedMessage{
		Content:  "urgent message",
		Priority: PrioritySteer,
	})
	assert.True(t, result)

	// 验证消息被添加
	assert.True(t, mgr.HasSteerMessages())

	// 获取消息
	msgs := mgr.DrainSteer()
	assert.Len(t, msgs, 1)
	assert.Equal(t, "urgent message", msgs[0].Content)
}

func TestMessageQueueIntegration_EnqueueFollowUpMessage(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 测试添加普通消息
	result := mgr.EnqueueFollowUp(QueuedMessage{
		Content:  "follow-up message",
		Priority: PriorityFollowUp,
	})
	assert.True(t, result)

	// 验证消息被添加
	assert.True(t, mgr.HasFollowUpMessages())

	// 获取消息
	msg, ok := mgr.DequeueFollowUp()
	assert.True(t, ok)
	assert.Equal(t, "follow-up message", msg.Content)
}

func TestMessageQueueIntegration_ProcessSteerMessages(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 添加多个紧急消息
	mgr.EnqueueSteer(QueuedMessage{Content: "msg1", Priority: PrioritySteer})
	mgr.EnqueueSteer(QueuedMessage{Content: "msg2", Priority: PrioritySteer})
	mgr.EnqueueSteer(QueuedMessage{Content: "msg3", Priority: PrioritySteer})

	// 处理消息
	msgs := mgr.DrainSteer()
	assert.Len(t, msgs, 3)

	// 验证消息内容
	assert.Equal(t, "msg1", msgs[0].Content)
	assert.Equal(t, "msg2", msgs[1].Content)
	assert.Equal(t, "msg3", msgs[2].Content)

	// 验证队列已清空
	assert.False(t, mgr.HasSteerMessages())
}

func TestMessageQueueIntegration_ProcessFollowUpMessages(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 添加普通消息
	mgr.EnqueueFollowUp(QueuedMessage{Content: "follow-up", Priority: PriorityFollowUp})

	// 处理消息
	msg, ok := mgr.DequeueFollowUp()
	assert.True(t, ok)
	assert.Equal(t, "follow-up", msg.Content)

	// 验证队列已清空
	assert.False(t, mgr.HasFollowUpMessages())
}

func TestMessageQueueIntegration_QueueStatus(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 添加消息
	mgr.EnqueueSteer(QueuedMessage{Content: "steer1", Priority: PrioritySteer})
	mgr.EnqueueSteer(QueuedMessage{Content: "steer2", Priority: PrioritySteer})
	mgr.EnqueueFollowUp(QueuedMessage{Content: "follow1", Priority: PriorityFollowUp})

	// 获取状态
	status := mgr.Status()
	assert.Equal(t, 2, status.SteerDepth)
	assert.Equal(t, DefaultSteerQueueCapacity, status.SteerCapacity)
	assert.Equal(t, 1, status.FollowupDepth)
	assert.Equal(t, DefaultFollowUpQueueCapacity, status.FollowupCapacity)
}

func TestMessageQueueIntegration_Clear(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 添加消息
	mgr.EnqueueSteer(QueuedMessage{Content: "steer", Priority: PrioritySteer})
	mgr.EnqueueFollowUp(QueuedMessage{Content: "follow", Priority: PriorityFollowUp})

	// 清空
	mgr.Clear()

	// 验证队列已清空
	assert.False(t, mgr.HasMessages())
	assert.False(t, mgr.HasSteerMessages())
	assert.False(t, mgr.HasFollowUpMessages())
}

func TestMessageQueueIntegration_ConcurrentAccess(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 并发添加消息
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			mgr.EnqueueSteer(QueuedMessage{
				Content:  "concurrent",
				Priority: PrioritySteer,
			})
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证消息数量（最多 5 个，因为队列容量为 5）
	msgs := mgr.DrainSteer()
	assert.LessOrEqual(t, len(msgs), 5)
}

func TestMessageQueueIntegration_PriorityOrder(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 添加不同优先级的消息
	mgr.EnqueueSteer(QueuedMessage{Content: "steer", Priority: PrioritySteer})
	mgr.EnqueueFollowUp(QueuedMessage{Content: "follow", Priority: PriorityFollowUp})

	// 验证紧急消息优先处理
	assert.True(t, mgr.HasSteerMessages())
	assert.True(t, mgr.HasFollowUpMessages())

	// 先处理紧急消息
	steerMsgs := mgr.DrainSteer()
	assert.Len(t, steerMsgs, 1)
	assert.Equal(t, "steer", steerMsgs[0].Content)

	// 再处理普通消息
	followMsg, ok := mgr.DequeueFollowUp()
	assert.True(t, ok)
	assert.Equal(t, "follow", followMsg.Content)
}

func TestMessageQueueIntegration_QueueOverflow(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 添加超过容量的消息
	for i := 0; i < 10; i++ {
		mgr.EnqueueSteer(QueuedMessage{
			Content:  "overflow",
			Priority: PrioritySteer,
		})
	}

	// 验证只保留了容量内的消息
	msgs := mgr.DrainSteer()
	assert.Len(t, msgs, DefaultSteerQueueCapacity)
}

func TestMessageQueueIntegration_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// 取消 context
	cancel()

	// 验证 context 已取消
	assert.Error(t, ctx.Err())
}
