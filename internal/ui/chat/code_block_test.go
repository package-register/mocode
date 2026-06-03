package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeBlockRenderer_Render(t *testing.T) {
	r := NewCodeBlockRenderer()

	block := CodeBlock{
		Language: "go",
		Content:  "fmt.Println(\"hello\")",
	}

	output := r.Render(block)
	assert.Contains(t, output, "go")
	assert.Contains(t, output, "[copy]")
	assert.Contains(t, output, "fmt.Println")
}

func TestCodeBlockRenderer_RenderNoLanguage(t *testing.T) {
	r := NewCodeBlockRenderer()

	block := CodeBlock{
		Content: "just some text",
	}

	output := r.Render(block)
	assert.Contains(t, output, "[copy]")
	assert.Contains(t, output, "just some text")
}

func TestCodeBlockRenderer_Copied(t *testing.T) {
	r := NewCodeBlockRenderer()

	block := CodeBlock{
		Language: "go",
		Content:  "fmt.Println(\"hello\")",
		Copied:   true,
	}

	output := r.Render(block)
	assert.Contains(t, output, "[copied!]")
	assert.NotContains(t, output, "[copy]")
}

func TestCodeBlockRenderer_Collapse(t *testing.T) {
	r := NewCodeBlockRenderer()

	// 创建一个超过 5 行的代码块
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7"
	block := CodeBlock{
		Language:  "go",
		Content:   content,
		Collapsed: true,
	}

	output := r.Render(block)
	assert.Contains(t, output, "...")
	assert.Contains(t, output, "lines hidden")
}

func TestCodeBlockRenderer_ShortContentNoCollapse(t *testing.T) {
	r := NewCodeBlockRenderer()

	// 短内容不应该显示折叠按钮
	content := "line1\nline2\nline3"
	block := CodeBlock{
		Language: "go",
		Content:  content,
	}

	output := r.Render(block)
	assert.NotContains(t, output, "collapse")
	assert.NotContains(t, output, "expand")
}

func TestCodeBlockRenderer_AddBlock(t *testing.T) {
	r := NewCodeBlockRenderer()

	block1 := CodeBlock{Language: "go", Content: "content1"}
	block2 := CodeBlock{Language: "python", Content: "content2"}

	r.AddBlock(block1)
	r.AddBlock(block2)

	blocks := r.GetBlocks()
	require.Len(t, blocks, 2)
	assert.Equal(t, "go", blocks[0].Language)
	assert.Equal(t, "python", blocks[1].Language)
	assert.Equal(t, 0, blocks[0].Index)
	assert.Equal(t, 1, blocks[1].Index)
}

func TestCodeBlockRenderer_GetBlock(t *testing.T) {
	r := NewCodeBlockRenderer()

	block := CodeBlock{Language: "go", Content: "content"}
	r.AddBlock(block)

	// 有效索引
	got := r.GetBlock(0)
	require.NotNil(t, got)
	assert.Equal(t, "go", got.Language)

	// 无效索引
	got = r.GetBlock(1)
	assert.Nil(t, got)

	got = r.GetBlock(-1)
	assert.Nil(t, got)
}

func TestCodeBlockRenderer_ClearBlocks(t *testing.T) {
	r := NewCodeBlockRenderer()

	r.AddBlock(CodeBlock{Content: "block1"})
	r.AddBlock(CodeBlock{Content: "block2"})

	assert.Len(t, r.GetBlocks(), 2)

	r.ClearBlocks()
	assert.Len(t, r.GetBlocks(), 0)
}

func TestCodeBlockRenderer_Clipboard(t *testing.T) {
	r := NewCodeBlockRenderer()

	// 初始剪贴板为空
	assert.Empty(t, r.GetClipboard())

	// 设置剪贴板
	r.clipboard = "test content"
	assert.Equal(t, "test content", r.GetClipboard())
}

func TestCodeBlockRenderer_Width(t *testing.T) {
	r := NewCodeBlockRenderer()

	assert.Equal(t, 80, r.width)

	r.SetWidth(120)
	assert.Equal(t, 120, r.width)
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Go function",
			content:  "func main() {\n    fmt.Println(\"hello\")\n}",
			expected: "go",
		},
		{
			name:     "Python function",
			content:  "def hello():\n    print(\"hello\")",
			expected: "python",
		},
		{
			name:     "JavaScript function",
			content:  "function hello() {\n    console.log(\"hello\")\n}",
			expected: "javascript",
		},
		{
			name:     "SQL query",
			content:  "SELECT * FROM users WHERE id = 1",
			expected: "sql",
		},
		{
			name:     "JSON",
			content:  "{\"key\": \"value\"}",
			expected: "json",
		},
		{
			name:     "HTML",
			content:  "<div>Hello</div>",
			expected: "html",
		},
		{
			name:     "Python shebang",
			content:  "#!/usr/bin/env python\nprint(\"hello\")",
			expected: "python",
		},
		{
			name:     "Bash shebang",
			content:  "#!/bin/bash\necho hello",
			expected: "bash",
		},
		{
			name:     "Unknown",
			content:  "just some text",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectLanguage(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCodeBlock(t *testing.T) {
	// 有语言
	result := FormatCodeBlock("fmt.Println(\"hello\")", "go")
	assert.Contains(t, result, "```go")
	assert.Contains(t, result, "fmt.Println")
	assert.Contains(t, result, "```")

	// 无语言
	result = FormatCodeBlock("just text", "")
	assert.Contains(t, result, "```")
	assert.Contains(t, result, "just text")
}

func TestCodeBlock_Fields(t *testing.T) {
	block := CodeBlock{
		Language:  "go",
		Content:   "content",
		StartLine: 10,
		EndLine:   20,
		Collapsed: true,
		Copied:    false,
		Index:     5,
	}

	assert.Equal(t, "go", block.Language)
	assert.Equal(t, "content", block.Content)
	assert.Equal(t, 10, block.StartLine)
	assert.Equal(t, 20, block.EndLine)
	assert.True(t, block.Collapsed)
	assert.False(t, block.Copied)
	assert.Equal(t, 5, block.Index)
}

func TestCodeBlockStyle_Default(t *testing.T) {
	style := DefaultCodeBlockStyle()

	assert.NotNil(t, style.HeaderStyle)
	assert.NotNil(t, style.LangStyle)
	assert.NotNil(t, style.CopyBtnStyle)
	assert.NotNil(t, style.CopiedStyle)
	assert.NotNil(t, style.ContentStyle)
	assert.NotNil(t, style.CollapseStyle)
	assert.NotNil(t, style.BorderStyle)
}

func TestCodeBlockCopiedMsg_Fields(t *testing.T) {
	msg := CodeBlockCopiedMsg{
		BlockIndex: 5,
		Content:    "test content",
	}

	assert.Equal(t, 5, msg.BlockIndex)
	assert.Equal(t, "test content", msg.Content)
}

func TestCodeBlockToggledMsg_Fields(t *testing.T) {
	msg := CodeBlockToggledMsg{
		BlockIndex: 3,
		Collapsed:  true,
	}

	assert.Equal(t, 3, msg.BlockIndex)
	assert.True(t, msg.Collapsed)
}
