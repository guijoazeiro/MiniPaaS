package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	tokenScopes          []string
	tokenExpiresAt       string
	tokenRevokeConfirmed bool
)

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Manage personal API tokens",
}

var tokensCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a personal API token",
	Long:  "Creates a token for automation. The secret is printed only once; save it in a password manager or MINIPAAS_TOKEN.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		created, err := apiClient.CreateAPIToken(args[0], tokenScopes, tokenExpiresAt)
		if err != nil {
			return err
		}
		fmt.Printf("created: %s  (%s)\n", created.Name, created.ID)
		fmt.Printf("scopes: %s\n", strings.Join(created.Scopes, ", "))
		if created.ExpiresAt != nil {
			fmt.Printf("expires: %s\n", *created.ExpiresAt)
		}
		fmt.Println("token (shown once; store it securely):")
		fmt.Println(created.Token)
		return nil
	},
}

var tokensListCmd = &cobra.Command{
	Use:   "list",
	Short: "List personal API tokens",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		tokens, err := apiClient.ListAPITokens()
		if err != nil {
			return err
		}
		if len(tokens) == 0 {
			fmt.Println("no API tokens")
			return nil
		}
		fmt.Printf("%-38s  %-20s  %-16s  %-24s  %s\n", "ID", "NAME", "SCOPES", "EXPIRES", "STATUS")
		for _, token := range tokens {
			status := "active"
			if token.RevokedAt != nil {
				status = "revoked"
			}
			expires := "never"
			if token.ExpiresAt != nil {
				expires = *token.ExpiresAt
			}
			fmt.Printf("%-38s  %-20s  %-16s  %-24s  %s\n", token.ID, token.Name, strings.Join(token.Scopes, ","), expires, status)
		}
		return nil
	},
}

var tokensRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke a personal API token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !tokenRevokeConfirmed {
			return fmt.Errorf("revoking an API token is permanent; re-run with --yes to confirm")
		}
		if err := apiClient.RevokeAPIToken(args[0]); err != nil {
			return err
		}
		fmt.Printf("revoked: %s\n", args[0])
		return nil
	},
}

func init() {
	tokensCreateCmd.Flags().StringSliceVar(&tokenScopes, "scope", nil, "token scope (repeat or separate with commas: read, deploy, manage)")
	tokensCreateCmd.Flags().StringVar(&tokenExpiresAt, "expires-at", "", "expiration timestamp in RFC3339 format")
	tokensRevokeCmd.Flags().BoolVarP(&tokenRevokeConfirmed, "yes", "y", false, "confirm permanent revocation")
	tokensCmd.AddCommand(tokensCreateCmd, tokensListCmd, tokensRevokeCmd)
	rootCmd.AddCommand(tokensCmd)
}
