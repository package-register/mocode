// Package notify defines domain notification types for agent events.
// These types are decoupled from UI concerns so the agent can publish
// events without importing UI packages.
package notify

// Type identifies the kind of agent notification.
type Type string

const (
	// TypeAgentThinking indicates the agent is processing/thinking.
	TypeAgentThinking Type = "agent_thinking"
	// TypeAgentToolExecuting indicates the agent is executing a tool.
	TypeAgentToolExecuting Type = "agent_tool_executing"
	// TypeAgentFinished indicates the agent has completed its turn.
	TypeAgentFinished Type = "agent_finished"
	// TypeReAuthenticate indicates the agent encountered an
	// authentication error and the user needs to re-authenticate.
	TypeReAuthenticate Type = "re_authenticate"
)

// Notification represents a domain event published by the agent.
type Notification struct {
	SessionID    string
	SessionTitle string
	Type         Type
	ProviderID   string
	ToolName     string // Name of the tool being executed (for TypeAgentToolExecuting)
}
