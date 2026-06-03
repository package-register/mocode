package authhandler

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/package-register/mocode/internal/config"
	"github.com/package-register/mocode/internal/minimax"
)

type minimaxHandler struct{}

func init() {
	Register(minimaxHandler{})
}

func (minimaxHandler) ID() string { return "minimax" }

func (minimaxHandler) Description() string { return "Authenticate MiniMax quota API key" }

func (minimaxHandler) Login(_ context.Context, env Env) error {
	apiKey := strings.TrimSpace(os.Getenv("MINIMAX_API_KEY"))
	if apiKey == "" {
		fmt.Fprint(env.Stdout, "MiniMax API Key: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		apiKey = strings.TrimSpace(line)
	}
	if apiKey == "" {
		return fmt.Errorf("MiniMax API key is empty")
	}

	provider := minimax.ProviderConfig(apiKey)
	switch {
	case env.Client != nil && env.WorkspaceID != "":
		if err := env.Client.SetConfigField(context.Background(), env.WorkspaceID, config.ScopeGlobal, "providers."+minimax.ProviderID, provider); err != nil {
			return err
		}
	case env.Store != nil:
		if err := env.Store.SetConfigField(config.ScopeGlobal, "providers."+minimax.ProviderID, provider); err != nil {
			return err
		}
	default:
		return fmt.Errorf("config store is required")
	}

	fmt.Fprintln(env.Stdout, "MiniMax authenticated.")
	fmt.Fprintln(env.Stdout, "Quota configuration updated.")
	return nil
}
