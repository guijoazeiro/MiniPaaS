package cmd

import (
	"os"

	"github.com/guijoazeiro/MiniPaaS/cli/internal/api"
	"github.com/spf13/cobra"
)

var (
	host      string
	apiClient *api.Client
)

var rootCmd = &cobra.Command{
	Use:   "minip",
	Short: "MiniPaaS CLI",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if h := os.Getenv("MINIPAAS_HOST"); h != "" && host == "http://localhost:8080" {
			host = h
		}
		apiClient = api.New(host)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&host, "host", "http://localhost:8080", "MiniPaaS API URL (or set MINIPAAS_HOST)")
	rootCmd.AddCommand(appsCmd, deployCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
