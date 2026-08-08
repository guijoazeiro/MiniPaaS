package cmd

import (
	"fmt"
	"os"

	"github.com/guijoazeiro/MiniPaaS/cli/internal/api"
	cfgpkg "github.com/guijoazeiro/MiniPaaS/cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	hostFlag  string
	apiClient *api.Client
	stored    *cfgpkg.Config
)

var rootCmd = &cobra.Command{
	Use:   "minip",
	Short: "MiniPaaS CLI",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		c, err := cfgpkg.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "warning: config load:", err)
			c = &cfgpkg.Config{}
		}
		stored = c

		host := hostFlag
		if host == "" {
			if v := os.Getenv("MINIPAAS_HOST"); v != "" {
				host = v
			} else if c.Host != "" {
				host = c.Host
			} else {
				host = "http://localhost:8080"
			}
		}
		apiClient = api.New(host, c.Token)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&hostFlag, "host", "", "MiniPaaS API URL (overrides saved config and MINIPAAS_HOST)")
	rootCmd.AddCommand(appsCmd, deployCmd, loginCmd, envCmd, logsCmd, rollbackCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
