// Package notify 定义了 agent 事件的领域通知类型。
//
// 本文件包含 notify 包的单元测试。
package notify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_Type_Constants 测试 Type 常量定义。
func Test_Type_Constants(t *testing.T) {
	tests := []struct {
		name     string
		got      Type
		expected string
	}{
		{"TypeAgentThinking", TypeAgentThinking, "agent_thinking"},
		{"TypeAgentToolExecuting", TypeAgentToolExecuting, "agent_tool_executing"},
		{"TypeAgentFinished", TypeAgentFinished, "agent_finished"},
		{"TypeReAuthenticate", TypeReAuthenticate, "re_authenticate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, Type(tt.expected), tt.got)
		})
	}
}

// Test_Notification_Creation 测试 Notification 结构体创建。
func Test_Notification_Creation(t *testing.T) {
	tests := []struct {
		name         string
		notification Notification
		wantSession  string
		wantType     Type
		wantTool     string
	}{
		{
			name: "思考状态通知",
			notification: Notification{
				SessionID: "session-123",
				Type:      TypeAgentThinking,
			},
			wantSession: "session-123",
			wantType:    TypeAgentThinking,
			wantTool:    "",
		},
		{
			name: "工具执行通知",
			notification: Notification{
				SessionID: "session-456",
				Type:      TypeAgentToolExecuting,
				ToolName:  "bash",
			},
			wantSession: "session-456",
			wantType:    TypeAgentToolExecuting,
			wantTool:    "bash",
		},
		{
			name: "完成通知",
			notification: Notification{
				SessionID:    "session-789",
				SessionTitle: "Test Session",
				Type:         TypeAgentFinished,
			},
			wantSession: "session-789",
			wantType:    TypeAgentFinished,
			wantTool:    "",
		},
		{
			name: "重新认证通知",
			notification: Notification{
				SessionID:  "session-abc",
				Type:       TypeReAuthenticate,
				ProviderID: "openai",
			},
			wantSession: "session-abc",
			wantType:    TypeReAuthenticate,
			wantTool:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantSession, tt.notification.SessionID)
			assert.Equal(t, tt.wantType, tt.notification.Type)
			assert.Equal(t, tt.wantTool, tt.notification.ToolName)
		})
	}
}

// Test_Notification_Fields 测试 Notification 字段的可选性。
func Test_Notification_Fields(t *testing.T) {
	// 最小化通知
	minimal := Notification{
		Type: TypeAgentThinking,
	}
	assert.Empty(t, minimal.SessionID)
	assert.Empty(t, minimal.SessionTitle)
	assert.Empty(t, minimal.ProviderID)
	assert.Empty(t, minimal.ToolName)

	// 完整通知
	full := Notification{
		SessionID:    "session-123",
		SessionTitle: "My Session",
		Type:         TypeAgentToolExecuting,
		ProviderID:   "anthropic",
		ToolName:     "grep",
	}
	assert.NotEmpty(t, full.SessionID)
	assert.NotEmpty(t, full.SessionTitle)
	assert.NotEmpty(t, full.ProviderID)
	assert.NotEmpty(t, full.ToolName)
}
