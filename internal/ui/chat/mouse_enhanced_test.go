package chat

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestEnhancedMouseHandler_SingleClick(t *testing.T) {
	h := NewEnhancedMouseHandler()

	// 单击
	cmd := h.HandleMouseClick(ansi.MouseLeft, 10, 10)
	// 单击可能返回 cmd（toggleExpand），这是正常的
	_ = cmd
	assert.Equal(t, 1, h.clickCount)
}

func TestEnhancedMouseHandler_DoubleClick(t *testing.T) {
	h := NewEnhancedMouseHandler()

	// 双击
	h.HandleMouseClick(ansi.MouseLeft, 10, 10)
	cmd := h.HandleMouseClick(ansi.MouseLeft, 10, 10)
	assert.Nil(t, cmd) // 没有可选择的单词
	assert.Equal(t, 2, h.clickCount)
}

func TestEnhancedMouseHandler_TripleClick(t *testing.T) {
	h := NewEnhancedMouseHandler()

	// 三击
	h.HandleMouseClick(ansi.MouseLeft, 10, 10)
	h.HandleMouseClick(ansi.MouseLeft, 10, 10)
	cmd := h.HandleMouseClick(ansi.MouseLeft, 10, 10)
	assert.Nil(t, cmd)               // 没有可选择的行
	assert.Equal(t, 0, h.clickCount) // 重置
}

func TestEnhancedMouseHandler_ClickTimeout(t *testing.T) {
	h := NewEnhancedMouseHandler()

	// 第一次点击
	h.HandleMouseClick(ansi.MouseLeft, 10, 10)
	assert.Equal(t, 1, h.clickCount)

	// 模拟超时（500ms 后）
	h.lastClickTime = time.Now().Add(-600 * time.Millisecond)

	// 第二次点击（应该重置为 1）
	h.HandleMouseClick(ansi.MouseLeft, 10, 10)
	assert.Equal(t, 1, h.clickCount)
}

func TestEnhancedMouseHandler_Drag(t *testing.T) {
	h := NewEnhancedMouseHandler()

	// 开始拖拽
	h.HandleMouseDrag(10, 10)
	assert.True(t, h.IsDragging())
	assert.Equal(t, Position{X: 10, Y: 10}, h.GetDragStart())

	// 拖拽中
	h.HandleMouseDrag(20, 15)
	assert.True(t, h.IsDragging())
	assert.Equal(t, Position{X: 20, Y: 15}, h.GetDragEnd())

	// 释放
	h.HandleMouseUp(20, 15)
	assert.False(t, h.IsDragging())
}

func TestEnhancedMouseHandler_Selection(t *testing.T) {
	h := NewEnhancedMouseHandler()

	// 初始状态
	assert.Nil(t, h.GetSelection())

	// 模拟拖拽选择
	h.HandleMouseDrag(10, 5)
	h.HandleMouseDrag(20, 5)
	h.HandleMouseUp(20, 5)

	// 应该有选择
	selection := h.GetSelection()
	assert.NotNil(t, selection)
}

func TestEnhancedMouseHandler_ClearSelection(t *testing.T) {
	h := NewEnhancedMouseHandler()

	// 模拟选择
	h.HandleMouseDrag(10, 5)
	h.HandleMouseDrag(20, 5)
	h.HandleMouseUp(20, 5)
	assert.NotNil(t, h.GetSelection())

	// 清除选择
	h.ClearSelection()
	assert.Nil(t, h.GetSelection())
}

func TestEnhancedMouseHandler_URLHover(t *testing.T) {
	h := NewEnhancedMouseHandler()

	// 初始状态
	assert.Nil(t, h.GetHoveredURL())

	// 设置悬停 URL
	url := &URLInfo{
		URL:   "https://example.com",
		Start: 10,
		End:   30,
		Line:  5,
		Col:   10,
	}
	h.SetHoveredURL(url)
	assert.NotNil(t, h.GetHoveredURL())
	assert.Equal(t, "https://example.com", h.GetHoveredURL().URL)

	// 移动鼠标（不在 URL 上）
	h.HandleMouseMotion(50, 50)
	assert.Nil(t, h.GetHoveredURL())
}

func TestEnhancedMouseHandler_Enabled(t *testing.T) {
	h := NewEnhancedMouseHandler()

	// 默认启用
	assert.True(t, h.enabled)

	// 禁用
	h.SetEnabled(false)
	assert.False(t, h.enabled)

	// 禁用时点击应该无效
	h.HandleMouseClick(ansi.MouseLeft, 10, 10)
	assert.Equal(t, 0, h.clickCount)
}

func TestEnhancedMouseHandler_RenderSelection(t *testing.T) {
	h := NewEnhancedMouseHandler()

	content := "Hello World\nThis is a test\nAnother line"

	// 没有选择
	result := h.RenderSelection(content, 80)
	assert.Equal(t, content, result)

	// 有选择
	h.selection = &Selection{
		Start:   Position{X: 0, Y: 0},
		End:     Position{X: 5, Y: 0},
		Content: "Hello",
	}
	result = h.RenderSelection(content, 80)
	assert.Contains(t, result, "Hello")
}

func TestPosition_Equal(t *testing.T) {
	p1 := Position{X: 10, Y: 20}
	p2 := Position{X: 10, Y: 20}
	p3 := Position{X: 30, Y: 40}

	assert.Equal(t, p1, p2)
	assert.NotEqual(t, p1, p3)
}

func TestURLInfo_Fields(t *testing.T) {
	url := URLInfo{
		URL:   "https://example.com",
		Start: 10,
		End:   30,
		Line:  5,
		Col:   10,
	}

	assert.Equal(t, "https://example.com", url.URL)
	assert.Equal(t, 10, url.Start)
	assert.Equal(t, 30, url.End)
	assert.Equal(t, 5, url.Line)
	assert.Equal(t, 10, url.Col)
}

func TestSelection_Fields(t *testing.T) {
	now := time.Now()
	sel := Selection{
		Start:     Position{X: 0, Y: 0},
		End:       Position{X: 10, Y: 0},
		Content:   "Hello World",
		StartTime: now,
	}

	assert.Equal(t, 0, sel.Start.X)
	assert.Equal(t, 10, sel.End.X)
	assert.Equal(t, "Hello World", sel.Content)
	assert.Equal(t, now, sel.StartTime)
}

func TestAbs(t *testing.T) {
	assert.Equal(t, 5, abs(5))
	assert.Equal(t, 5, abs(-5))
	assert.Equal(t, 0, abs(0))
}

func TestMin(t *testing.T) {
	assert.Equal(t, 5, min(5, 10))
	assert.Equal(t, 5, min(10, 5))
	assert.Equal(t, 5, min(5, 5))
}

func TestMax(t *testing.T) {
	assert.Equal(t, 10, max(5, 10))
	assert.Equal(t, 10, max(10, 5))
	assert.Equal(t, 5, max(5, 5))
}
