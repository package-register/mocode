package agent

import (
	"sync"

	"github.com/package-register/mocode/internal/session/message"
)

// MessagePriority 消息优先级
type MessagePriority int

const (
	// PrioritySteer 紧急消息，中-turn 注入
	PrioritySteer MessagePriority = iota
	// PriorityFollowUp 普通消息，turn 结束后注入
	PriorityFollowUp
)

// QueuedMessage 是等待注入到 agent 循环的消息
type QueuedMessage struct {
	Content      string
	MultiContent []message.ContentPart
	Priority     MessagePriority
}

// QueueStatus 表示队列的当前状态
type QueueStatus struct {
	SteerDepth       int
	SteerCapacity    int
	FollowupDepth    int
	FollowupCapacity int
}

// MessageQueue 消息队列接口
type MessageQueue interface {
	// Enqueue 添加消息到队列，返回 false 如果队列已满
	Enqueue(msg QueuedMessage) bool
	// Dequeue 移除并返回队列中的下一个消息
	Dequeue() (QueuedMessage, bool)
	// Drain 返回所有待处理消息并清空队列
	Drain() []QueuedMessage
	// Len 返回队列中的消息数量
	Len() int
	// Cap 返回队列容量
	Cap() int
}

// inMemoryQueue 基于 channel 的内存队列实现
type inMemoryQueue struct {
	ch chan QueuedMessage
}

// newInMemoryQueue 创建指定容量的内存队列
func newInMemoryQueue(capacity int) *inMemoryQueue {
	return &inMemoryQueue{ch: make(chan QueuedMessage, capacity)}
}

func (q *inMemoryQueue) Enqueue(msg QueuedMessage) bool {
	select {
	case q.ch <- msg:
		return true
	default:
		return false // 队列已满
	}
}

func (q *inMemoryQueue) Dequeue() (QueuedMessage, bool) {
	select {
	case msg := <-q.ch:
		return msg, true
	default:
		return QueuedMessage{}, false // 队列为空
	}
}

func (q *inMemoryQueue) Drain() []QueuedMessage {
	var msgs []QueuedMessage
	for {
		select {
		case msg := <-q.ch:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

func (q *inMemoryQueue) Len() int {
	return len(q.ch)
}

func (q *inMemoryQueue) Cap() int {
	return cap(q.ch)
}

// MessageQueueManager 管理两种消息队列
type MessageQueueManager struct {
	steerQueue    MessageQueue
	followUpQueue MessageQueue
	mu            sync.RWMutex
}

const (
	// DefaultSteerQueueCapacity 默认紧急队列容量
	DefaultSteerQueueCapacity = 5
	// DefaultFollowUpQueueCapacity 默认普通队列容量
	DefaultFollowUpQueueCapacity = 20
)

// NewMessageQueueManager 创建消息队列管理器
func NewMessageQueueManager() *MessageQueueManager {
	return &MessageQueueManager{
		steerQueue:    newInMemoryQueue(DefaultSteerQueueCapacity),
		followUpQueue: newInMemoryQueue(DefaultFollowUpQueueCapacity),
	}
}

// NewMessageQueueManagerWithCapacity 创建指定容量的消息队列管理器
func NewMessageQueueManagerWithCapacity(steerCap, followUpCap int) *MessageQueueManager {
	return &MessageQueueManager{
		steerQueue:    newInMemoryQueue(steerCap),
		followUpQueue: newInMemoryQueue(followUpCap),
	}
}

// EnqueueSteer 添加紧急消息
func (m *MessageQueueManager) EnqueueSteer(msg QueuedMessage) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg.Priority = PrioritySteer
	return m.steerQueue.Enqueue(msg)
}

// EnqueueFollowUp 添加普通消息
func (m *MessageQueueManager) EnqueueFollowUp(msg QueuedMessage) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg.Priority = PriorityFollowUp
	return m.followUpQueue.Enqueue(msg)
}

// Enqueue 根据优先级添加消息
func (m *MessageQueueManager) Enqueue(msg QueuedMessage) bool {
	if msg.Priority == PrioritySteer {
		return m.EnqueueSteer(msg)
	}
	return m.EnqueueFollowUp(msg)
}

// DrainSteer 获取所有紧急消息
func (m *MessageQueueManager) DrainSteer() []QueuedMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.steerQueue.Drain()
}

// DequeueFollowUp 获取一个普通消息
func (m *MessageQueueManager) DequeueFollowUp() (QueuedMessage, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.followUpQueue.Dequeue()
}

// Status 获取队列状态
func (m *MessageQueueManager) Status() QueueStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return QueueStatus{
		SteerDepth:       m.steerQueue.Len(),
		SteerCapacity:    m.steerQueue.Cap(),
		FollowupDepth:    m.followUpQueue.Len(),
		FollowupCapacity: m.followUpQueue.Cap(),
	}
}

// Clear 清空所有队列
func (m *MessageQueueManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.steerQueue.Drain()
	m.followUpQueue.Drain()
}

// HasMessages 检查是否有待处理消息
func (m *MessageQueueManager) HasMessages() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.steerQueue.Len() > 0 || m.followUpQueue.Len() > 0
}

// HasSteerMessages 检查是否有紧急消息
func (m *MessageQueueManager) HasSteerMessages() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.steerQueue.Len() > 0
}

// HasFollowUpMessages 检查是否有普通消息
func (m *MessageQueueManager) HasFollowUpMessages() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.followUpQueue.Len() > 0
}
