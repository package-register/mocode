package capability

import "github.com/package-register/mocode/internal/commands"

type CommandCategory string

const (
	CommandCategorySystem CommandCategory = "system"
	CommandCategoryUser   CommandCategory = "user"
	CommandCategoryMCP    CommandCategory = "mcp"
)

type RiskLevel string

const (
	RiskLevelRead        RiskLevel = "read"
	RiskLevelWrite       RiskLevel = "write"
	RiskLevelNetwork     RiskLevel = "network"
	RiskLevelDestructive RiskLevel = "destructive"
)

type ProviderKind string

const (
	ProviderKindBuiltin       ProviderKind = "builtin"
	ProviderKindCustomCommand ProviderKind = "custom-command"
	ProviderKindMCP           ProviderKind = "mcp"
	ProviderKindSession       ProviderKind = "session"
)

type ProviderInfo struct {
	ID      string
	Name    string
	Kind    ProviderKind
	Source  string
	Version string
}

type Diagnostic struct {
	ProviderID string
	Message    string
	Err        error
}

type CommandContext struct{}

type CommandDescriptor struct {
	ID          string
	Title       string
	Shortcut    string
	Description string
	Category    CommandCategory
	Arguments   []commands.Argument
	Risk        RiskLevel
	Provider    ProviderInfo
	Action      any
}

type CommandProvider interface {
	ProviderInfo() ProviderInfo
	Commands(CommandContext) ([]CommandDescriptor, error)
}

type StaticCommandProvider struct {
	Info  ProviderInfo
	Items []CommandDescriptor
}

func (p StaticCommandProvider) ProviderInfo() ProviderInfo {
	return p.Info
}

func (p StaticCommandProvider) Commands(CommandContext) ([]CommandDescriptor, error) {
	items := make([]CommandDescriptor, len(p.Items))
	copy(items, p.Items)
	for i := range items {
		if items[i].Provider.ID == "" {
			items[i].Provider = p.Info
		}
	}
	return items, nil
}

type CommandRegistry struct {
	providers []CommandProvider
}

func NewCommandRegistry(providers ...CommandProvider) *CommandRegistry {
	registry := &CommandRegistry{}
	registry.providers = append(registry.providers, providers...)
	return registry
}

func (r *CommandRegistry) Commands(ctx CommandContext) ([]CommandDescriptor, []Diagnostic) {
	if r == nil {
		return nil, nil
	}
	var out []CommandDescriptor
	var diagnostics []Diagnostic
	for _, provider := range r.providers {
		if provider == nil {
			continue
		}
		items, err := provider.Commands(ctx)
		if err != nil {
			info := provider.ProviderInfo()
			diagnostics = append(diagnostics, Diagnostic{ProviderID: info.ID, Message: err.Error(), Err: err})
			continue
		}
		out = append(out, items...)
	}
	return out, diagnostics
}
