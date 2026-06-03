package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/package-register/mocode/internal/authhandler"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Aliases: []string{"login"},
	Use:     "auth [wechat|minimax]",
	Short:   "Authenticate mocode integrations",
	Long: `Authenticate mocode with a supported integration.
Available platforms are: wechat, minimax.`,
	Example: `
# Authenticate WeChat bot
mocode auth wechat

# Authenticate MiniMax quota API
mocode auth minimax
  `,
	ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return authhandler.IDs(), cobra.ShellCompDirectiveNoFileComp
	},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return nil
		}
		_ = cmd.Help()
		return fmt.Errorf("platform required: %s", strings.Join(authhandler.IDs(), ", "))
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		c, ws, cleanup, err := connectToServer(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		progressEnabled := ws.Config.Options.Progress == nil || *ws.Config.Options.Progress
		if progressEnabled && supportsProgressBar() {
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
			defer func() { _, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar) }()
		}

		provider := args[0]

		handler, ok := authhandler.Get(provider)
		if !ok {
			return fmt.Errorf("unknown platform: %s (available: %v)", provider, authhandler.IDs())
		}

		return handler.Login(getLoginContext(), authhandler.Env{
			Client:      c,
			WorkspaceID: ws.ID,
			Stdout:      os.Stdout,
			Stderr:      os.Stderr,
		})
	},
}

func getLoginContext() context.Context {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	go func() {
		<-ctx.Done()
		cancel()
		os.Exit(1)
	}()
	return ctx
}

func waitEnter() {
	_, _ = fmt.Scanln()
}
