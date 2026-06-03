package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueueIndicator_View_Empty(t *testing.T) {
	qi := NewQueueIndicator()

	// 空队列应该返回空字符串
	qi.Update(QueueStatus{})
	assert.Equal(t, "", qi.View())
}

func TestQueueIndicator_View_SteerOnly(t *testing.T) {
	qi := NewQueueIndicator()

	qi.Update(QueueStatus{
		SteerDepth:    2,
		SteerCapacity: 5,
	})

	// 设置 animFrame 为奇数，显示 ⚡
	qi.animFrame = 1
	view := qi.View()
	assert.Contains(t, view, "2/5")
	assert.Contains(t, view, "⚡")
}

func TestQueueIndicator_View_FollowUpOnly(t *testing.T) {
	qi := NewQueueIndicator()

	qi.Update(QueueStatus{
		FollowupDepth:    3,
		FollowupCapacity: 20,
	})

	view := qi.View()
	assert.Contains(t, view, "3/20")
	assert.Contains(t, view, "📨")
}

func TestQueueIndicator_View_BothQueues(t *testing.T) {
	qi := NewQueueIndicator()

	qi.Update(QueueStatus{
		SteerDepth:       1,
		SteerCapacity:    5,
		FollowupDepth:    2,
		FollowupCapacity: 20,
	})

	// 设置 animFrame 为奇数，显示 ⚡
	qi.animFrame = 1
	view := qi.View()
	assert.Contains(t, view, "1/5")
	assert.Contains(t, view, "2/20")
	assert.Contains(t, view, "⚡")
	assert.Contains(t, view, "📨")
}

func TestQueueIndicator_Visible(t *testing.T) {
	qi := NewQueueIndicator()

	qi.Update(QueueStatus{
		SteerDepth:    1,
		SteerCapacity: 5,
	})

	// 可见时应该显示
	qi.SetVisible(true)
	assert.NotEmpty(t, qi.View())

	// 不可见时应该隐藏
	qi.SetVisible(false)
	assert.Equal(t, "", qi.View())
}

func TestQueueIndicator_HasMessages(t *testing.T) {
	qi := NewQueueIndicator()

	// 空队列
	qi.Update(QueueStatus{})
	assert.False(t, qi.HasMessages())

	// 有紧急消息
	qi.Update(QueueStatus{SteerDepth: 1})
	assert.True(t, qi.HasMessages())

	// 有普通消息
	qi.Update(QueueStatus{FollowupDepth: 1})
	assert.True(t, qi.HasMessages())

	// 两种都有
	qi.Update(QueueStatus{SteerDepth: 1, FollowupDepth: 1})
	assert.True(t, qi.HasMessages())
}

func TestQueueIndicator_Animation(t *testing.T) {
	qi := NewQueueIndicator()

	qi.Update(QueueStatus{
		SteerDepth:    1,
		SteerCapacity: 5,
	})

	// 初始帧
	qi.animFrame = 0
	view1 := qi.View()

	// 下一帧
	qi.animFrame = 1
	view2 := qi.View()

	// 动画应该改变图标
	assert.NotEqual(t, view1, view2)
}

func TestQueueIndicator_StartAnimation(t *testing.T) {
	qi := NewQueueIndicator()

	// 空队列不应该启动动画
	qi.Update(QueueStatus{})
	cmd := qi.StartAnimation()
	assert.Nil(t, cmd)

	// 有消息应该启动动画
	qi.Update(QueueStatus{SteerDepth: 1})
	cmd = qi.StartAnimation()
	assert.NotNil(t, cmd)
}

func TestQueueIndicator_Width(t *testing.T) {
	qi := NewQueueIndicator()

	qi.SetWidth(80)
	assert.Equal(t, 80, qi.width)
}

func TestDefaultQueueIndicatorStyle(t *testing.T) {
	style := DefaultQueueIndicatorStyle()

	assert.NotNil(t, style.SteerStyle)
	assert.NotNil(t, style.FollowupStyle)
	assert.NotNil(t, style.BorderStyle)
	assert.NotEmpty(t, style.Separator)
}
