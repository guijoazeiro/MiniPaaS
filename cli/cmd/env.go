package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage app environment variables (encrypted at rest)",
}

var envListCmd = &cobra.Command{
	Use:   "list <app>",
	Short: "List env keys for an app (values are never shown)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keys, err := apiClient.ListEnv(args[0])
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			fmt.Println("(no env vars)")
			return nil
		}
		for _, k := range keys {
			fmt.Printf("%s\t(updated %s)\n", k.Key, k.UpdatedAt)
		}
		return nil
	},
}

var envSetCmd = &cobra.Command{
	Use:   "set <app> KEY=value [KEY=value ...]",
	Short: "Set one or more env vars",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := args[0]
		for _, kv := range args[1:] {
			key, val, ok := strings.Cut(kv, "=")
			if !ok || key == "" {
				return fmt.Errorf("invalid pair %q (expected KEY=value)", kv)
			}
			if err := apiClient.SetEnv(app, key, val); err != nil {
				return fmt.Errorf("set %s: %w", key, err)
			}
			fmt.Println("set", key)
		}
		return nil
	},
}

var envUnsetCmd = &cobra.Command{
	Use:   "unset <app> <key>",
	Short: "Remove an env var",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := apiClient.UnsetEnv(args[0], args[1]); err != nil {
			return err
		}
		fmt.Println("removed", args[1])
		return nil
	},
}

func init() {
	envCmd.AddCommand(envListCmd, envSetCmd, envUnsetCmd)
}
