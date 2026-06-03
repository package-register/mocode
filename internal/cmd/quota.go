package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/package-register/mocode/internal/config"
	"github.com/package-register/mocode/internal/minimax"
	"github.com/spf13/cobra"
)

var quotaCmd = &cobra.Command{
	Use:   "quota",
	Short: "查看 MiniMax API 配额用量",
	Long:  "查询已配置的 MiniMax provider 的 API 配额使用情况，包括周期用量、周用量、起止时间和重置倒计时。",
	Example: `# 查看所有 MiniMax provider 配额
mocode quota`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		dataDir, _ := cmd.Flags().GetString("data-dir")
		debug, _ := cmd.Flags().GetBool("debug")

		store, err := config.Init(cwd, dataDir, debug)
		if err != nil {
			return err
		}

		if !store.Config().IsConfigured() {
			return fmt.Errorf("no providers configured - please run 'mocode' to set up a provider interactively")
		}

		found := false
		for providerID, provider := range store.Config().Providers.Seq2() {
			if provider.Disable {
				continue
			}

			if !minimax.IsProvider(providerID, provider) {
				continue
			}

			regionBaseURL := minimax.QuotaBaseURL(provider)

			apiKey := strings.TrimSpace(provider.APIKey)
			if apiKey == "" {
				fmt.Printf("provider %q: API key not configured, skipping\n", providerID)
				continue
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()

			client := minimax.NewMiniMaxQuotaClient(apiKey, regionBaseURL)
			client.SetHTTPClient(store.Config().HTTPClient(store.Resolver(), 15*time.Second))
			if value := minimax.QuotaCookie(provider); value != "" {
				client.SetCookie(value)
			}
			resp, err := client.FetchQuota(ctx)
			if err != nil {
				fmt.Printf("provider %q: query failed: %v\n", providerID, err)
				continue
			}

			fmt.Printf("provider: %s (%s)\n", providerID, regionBaseURL)
			fmt.Print(minimax.FormatQuotaTable(resp.ModelRemain))
			found = true
		}

		if !found {
			return fmt.Errorf("no MiniMax providers found - configure one with 'mocode' interactive setup")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(quotaCmd)
}
