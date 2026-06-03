package agent

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageQueueManager_SteerQueue(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 测试入队
	assert.True(t, mgr.EnqueueSteer(QueuedMessage{Content: "steer-1"}))
	assert.True(t, mgr.EnqueueSteer(QueuedMessage{Content: "steer-2"}))

	// 测试 Drain
	msgs := mgr.DrainSteer()
	require.Len(t, msgs, 2)
	assert.Equal(t, "steer-1", msgs[0].Content)
	assert.Equal(t, "steer-2", msgs[1].Content)

	// Drain 后队列应该为空
	msgs = mgr.DrainSteer()
	assert.Len(t, msgs, 0)
}

func TestMessageQueueManager_FollowUpQueue(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 测试入队
	assert.True(t, mgr.EnqueueFollowUp(QueuedMessage{Content: "follow-1"}))
	assert.True(t, mgr.EnqueueFollowUp(QueuedMessage{Content: "follow-2"}))

	// 测试 Dequeue（先进先出）
	msg, ok := mgr.DequeueFollowUp()
	require.True(t, ok)
	assert.Equal(t, "follow-1", msg.Content)

	msg, ok = mgr.DequeueFollowUp()
	require.True(t, ok)
	assert.Equal(t, "follow-2", msg.Content)

	// 队列应该为空
	msg, ok = mgr.DequeueFollowUp()
	assert.False(t, ok)
}

func TestMessageQueueManager_QueueOverflow(t *testing.T) {
	mgr := NewMessageQueueManagerWithCapacity(3, 5)

	// 测试紧急队列溢出
	for i := 0; i < 5; i++ {
		mgr.EnqueueSteer(QueuedMessage{Content: fmt.Sprintf("steer-%d", i)})
	}

	// 只有前 3 条消息被保留
	msgs := mgr.DrainSteer()
	assert.Len(t, msgs, 3)
	assert.Equal(t, "steer-0", msgs[0].Content)
	assert.Equal(t, "steer-1", msgs[1].Content)
	assert.Equal(t, "steer-2", msgs[2].Content)

	// 测试普通队列溢出
	for i := 0; i < 10; i++ {
		mgr.EnqueueFollowUp(QueuedMessage{Content: fmt.Sprintf("follow-%d", i)})
	}

	// 只有前 5 条消息被保留
	count := 0
	for {
		_, ok := mgr.DequeueFollowUp()
		if !ok {
			break
		}
		count++
	}
	assert.Equal(t, 5, count)
}

func TestMessageQueueManager_Status(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 初始状态
	status := mgr.Status()
	assert.Equal(t, 0, status.SteerDepth)
	assert.Equal(t, DefaultSteerQueueCapacity, status.SteerCapacity)
	assert.Equal(t, 0, status.FollowupDepth)
	assert.Equal(t, DefaultFollowUpQueueCapacity, status.FollowupCapacity)

	// 添加消息后
	mgr.EnqueueSteer(QueuedMessage{Content: "steer"})
	mgr.EnqueueFollowUp(QueuedMessage{Content: "follow"})

	status = mgr.Status()
	assert.Equal(t, 1, status.SteerDepth)
	assert.Equal(t, 1, status.FollowupDepth)
}

func TestMessageQueueManager_Clear(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 添加消息
	mgr.EnqueueSteer(QueuedMessage{Content: "steer"})
	mgr.EnqueueFollowUp(QueuedMessage{Content: "follow"})

	// 清空
	mgr.Clear()

	// 验证队列为空
	status := mgr.Status()
	assert.Equal(t, 0, status.SteerDepth)
	assert.Equal(t, 0, status.FollowupDepth)
}

func TestMessageQueueManager_HasMessages(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 初始状态
	assert.False(t, mgr.HasMessages())
	assert.False(t, mgr.HasSteerMessages())
	assert.False(t, mgr.HasFollowUpMessages())

	// 添加紧急消息
	mgr.EnqueueSteer(QueuedMessage{Content: "steer"})
	assert.True(t, mgr.HasMessages())
	assert.True(t, mgr.HasSteerMessages())
	assert.False(t, mgr.HasFollowUpMessages())

	// 添加普通消息
	mgr.EnqueueFollowUp(QueuedMessage{Content: "follow"})
	assert.True(t, mgr.HasMessages())
	assert.True(t, mgr.HasSteerMessages())
	assert.True(t, mgr.HasFollowUpMessages())
}

func TestMessageQueueManager_EnqueueWithPriority(t *testing.T) {
	mgr := NewMessageQueueManager()

	// 使用 Enqueue 方法根据优先级添加
	mgr.Enqueue(QueuedMessage{Content: "steer", Priority: PrioritySteer})
	mgr.Enqueue(QueuedMessage{Content: "follow", Priority: PriorityFollowUp})

	// 验证消息被正确分配到队列
	steerMsgs := mgr.DrainSteer()
	assert.Len(t, steerMsgs, 1)
	assert.Equal(t, "steer", steerMsgs[0].Content)

	followMsg, ok := mgr.DequeueFollowUp()
	assert.True(t, ok)
	assert.Equal(t, "follow", followMsg.Content)
}

func TestMessageQueueManager_Concurrent(t *testing.T) {
	mgr := NewMessageQueueManagerWithCapacity(100, 100)

	var wg sync.WaitGroup

	// 并发写入紧急队列
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mgr.EnqueueSteer(QueuedMessage{Content: fmt.Sprintf("steer-%d", idx)})
		}(i)
	}

	// 并发写入普通队列
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mgr.EnqueueFollowUp(QueuedMessage{Content: fmt.Sprintf("follow-%d", idx)})
		}(i)
	}

	wg.Wait()

	// 验证所有消息都被正确写入
	status := mgr.Status()
	assert.Equal(t, 50, status.SteerDepth)
	assert.Equal(t, 50, status.FollowupDepth)
}

func TestInMemoryQueue_BasicOperations(t *testing.T) {
	q := newInMemoryQueue(3)

	// 测试入队
	assert.True(t, q.Enqueue(QueuedMessage{Content: "msg-1"}))
	assert.True(t, q.Enqueue(QueuedMessage{Content: "msg-2"}))
	assert.True(t, q.Enqueue(QueuedMessage{Content: "msg-3"}))

	// 队列已满
	assert.False(t, q.Enqueue(QueuedMessage{Content: "msg-4"}))

	// 测试 Len 和 Cap
	assert.Equal(t, 3, q.Len())
	assert.Equal(t, 3, q.Cap())

	// 测试 Drain
	msgs := q.Drain()
	assert.Len(t, msgs, 3)
	assert.Equal(t, "msg-1", msgs[0].Content)
	assert.Equal(t, "msg-2", msgs[1].Content)
	assert.Equal(t, "msg-3", msgs[2].Content)

	// Drain 后队列应该为空
	assert.Equal(t, 0, q.Len())
}

func TestInMemoryQueue_DequeueEmpty(t *testing.T) {
	q := newInMemoryQueue(3)

	// 从空队列 Dequeue
	msg, ok := q.Dequeue()
	assert.False(t, ok)
	assert.Empty(t, msg.Content)
}
