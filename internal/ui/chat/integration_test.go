package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_EnhancedMouseHandler_WithURLDetector(t *testing.T) {
	// 测试增强鼠标处理器与 URL 检测器的集成
	handler := NewEnhancedMouseHandler()
	detector := NewURLDetector()

	// 检测 URL
	content := "Visit https://example.com for details"
	urls := detector.Detect(content)
	require.GreaterOrEqual(t, len(urls), 1)

	// 设置悬停 URL
	handler.SetHoveredURL(&urls[0])
	assert.NotNil(t, handler.GetHoveredURL())
	assert.Equal(t, "https://example.com", handler.GetHoveredURL().URL)

	// 移动鼠标（不在 URL 上）
	handler.HandleMouseMotion(50, 50)
	assert.Nil(t, handler.GetHoveredURL())
}

func TestIntegration_CodeBlockRenderer_WithDetectLanguage(t *testing.T) {
	// 测试代码块渲染器与语言检测的集成
	renderer := NewCodeBlockRenderer()

	// 自动检测语言
	goCode := "func main() {\n    fmt.Println(\"hello\")\n}"
	lang := DetectLanguage(goCode)
	assert.Equal(t, "go", lang)

	// 添加代码块
	renderer.AddBlock(CodeBlock{
		Language: lang,
		Content:  goCode,
	})

	blocks := renderer.GetBlocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "go", blocks[0].Language)
}

func TestIntegration_URLDetector_WithURLType(t *testing.T) {
	// 测试 URL 检测器与 URL 类型的集成
	detector := NewURLDetector()

	tests := []struct {
		content  string
		expected URLType
	}{
		{"https://example.com", URLTypeWeb},
		{"user@example.com", URLTypeEmail},
		{"/path/to/file", URLTypeFile},
		{"192.168.1.1", URLTypeIP},
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			urls := detector.Detect(tt.content)
			require.GreaterOrEqual(t, len(urls), 1)

			urlType := detector.GetURLType(urls[0].URL)
			assert.Equal(t, tt.expected, urlType)
		})
	}
}

func TestIntegration_CodeBlockRenderer_CollapseExpand(t *testing.T) {
	// 测试代码块折叠/展开的完整流程
	renderer := NewCodeBlockRenderer()

	// 创建长代码块
	content := ""
	for i := 0; i < 10; i++ {
		content += "line\n"
	}

	renderer.AddBlock(CodeBlock{
		Language:  "go",
		Content:   content,
		Collapsed: false,
	})

	// 初始状态（未折叠）
	blocks := renderer.GetBlocks()
	require.Len(t, blocks, 1)
	assert.False(t, blocks[0].Collapsed)

	// 折叠
	renderer.toggleCollapse(0)
	blocks = renderer.GetBlocks()
	assert.True(t, blocks[0].Collapsed)

	// 展开
	renderer.toggleCollapse(0)
	blocks = renderer.GetBlocks()
	assert.False(t, blocks[0].Collapsed)
}

func TestIntegration_CodeBlockRenderer_CopyToClipboard(t *testing.T) {
	// 测试代码块复制到剪贴板的完整流程
	renderer := NewCodeBlockRenderer()

	renderer.AddBlock(CodeBlock{
		Language: "go",
		Content:  "fmt.Println(\"hello\")",
	})

	// 初始剪贴板为空
	assert.Empty(t, renderer.GetClipboard())

	// 复制
	renderer.copyToClipboard(0)

	// 验证剪贴板内容
	assert.Equal(t, "fmt.Println(\"hello\")", renderer.GetClipboard())

	// 验证代码块状态
	blocks := renderer.GetBlocks()
	assert.True(t, blocks[0].Copied)
}

func TestIntegration_EnhancedMouseHandler_DragSelection(t *testing.T) {
	// 测试拖拽选择的完整流程
	handler := NewEnhancedMouseHandler()

	// 开始拖拽
	handler.HandleMouseDrag(10, 5)
	assert.True(t, handler.IsDragging())

	// 拖拽中
	handler.HandleMouseDrag(20, 10)
	assert.True(t, handler.IsDragging())

	// 释放
	handler.HandleMouseUp(20, 10)
	assert.False(t, handler.IsDragging())

	// 应该有选择
	selection := handler.GetSelection()
	assert.NotNil(t, selection)
}

func TestIntegration_URLDetector_MultiplePatterns(t *testing.T) {
	// 测试多个 URL 模式的集成
	detector := NewURLDetector()

	// 添加自定义模式
	err := detector.AddPattern(`#\d+`)
	require.NoError(t, err)

	content := "Fix #123 and visit https://example.com"
	urls := detector.Detect(content)

	// 应该检测到两种类型的 URL
	foundIssue := false
	foundURL := false
	for _, u := range urls {
		if u.URL == "#123" {
			foundIssue = true
		}
		if u.URL == "https://example.com" {
			foundURL = true
		}
	}
	assert.True(t, foundIssue)
	assert.True(t, foundURL)
}

func TestIntegration_CodeBlockRenderer_MultipleBlocks(t *testing.T) {
	// 测试多个代码块的集成
	renderer := NewCodeBlockRenderer()

	renderer.AddBlock(CodeBlock{
		Language: "go",
		Content:  "fmt.Println(\"hello\")",
	})

	renderer.AddBlock(CodeBlock{
		Language: "python",
		Content:  "print(\"hello\")",
	})

	renderer.AddBlock(CodeBlock{
		Language: "javascript",
		Content:  "console.log(\"hello\")",
	})

	blocks := renderer.GetBlocks()
	assert.Len(t, blocks, 3)
	assert.Equal(t, "go", blocks[0].Language)
	assert.Equal(t, "python", blocks[1].Language)
	assert.Equal(t, "javascript", blocks[2].Language)
}
