package minimax

import (
	"strings"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/package-register/mocode/internal/config"
)

const (
	ProviderID          = "minimax"
	DefaultBaseURL      = "https://api.minimaxi.com/v1"
	DefaultQuotaBaseURL = "https://api.minimaxi.com"
	ProviderDisplayName = "MiniMax"
	ProviderTypeOpenAI  = catwalk.TypeOpenAICompat
	quotaBaseURLOption  = "quota_base_url"
	quotaCookieOption   = "quota_cookie"
)

// IsProvider reports whether a provider config is a MiniMax provider.
func IsProvider(providerID string, provider config.ProviderConfig) bool {
	idLower := strings.ToLower(providerID)
	if strings.Contains(idLower, "minimax") {
		return true
	}
	catwalkID := catwalk.InferenceProvider(providerID)
	if catwalkID == catwalk.InferenceProviderMiniMax || catwalkID == catwalk.InferenceProviderMiniMaxChina {
		return true
	}
	if strings.Contains(strings.ToLower(provider.BaseURL), "minimax") {
		return true
	}
	if v, ok := provider.ProviderOptions[quotaBaseURLOption].(string); ok && strings.Contains(strings.ToLower(v), "minimax") {
		return true
	}
	return false
}

// QuotaBaseURL returns the quota API base URL for the provider.
func QuotaBaseURL(provider config.ProviderConfig) string {
	regionBaseURL := DefaultQuotaBaseURL
	if strings.Contains(provider.BaseURL, "minimax.io") {
		regionBaseURL = "https://api.minimax.io"
	}
	if value, ok := provider.ProviderOptions[quotaBaseURLOption].(string); ok && strings.TrimSpace(value) != "" {
		regionBaseURL = strings.TrimSpace(value)
	}
	return regionBaseURL
}

// QuotaCookie returns the optional quota cookie for the provider.
func QuotaCookie(provider config.ProviderConfig) string {
	if value, ok := provider.ProviderOptions[quotaCookieOption].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

// ProviderConfig returns the canonical MiniMax provider configuration used by
// auth minimax and quota lookup. This is the single source for MiniMax quota
// credentials.
func ProviderConfig(apiKey string) config.ProviderConfig {
	return config.ProviderConfig{
		ID:      ProviderID,
		Name:    ProviderDisplayName,
		BaseURL: DefaultBaseURL,
		Type:    ProviderTypeOpenAI,
		APIKey:  strings.TrimSpace(apiKey),
		ProviderOptions: map[string]any{
			quotaBaseURLOption: DefaultQuotaBaseURL,
		},
	}
}
