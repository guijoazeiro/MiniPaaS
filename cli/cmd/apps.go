package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "Manage apps",
}

var appsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := apiClient.CreateApp(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("created: %s  (%s)  status=%s\n", app.Name, app.ID, app.Status)
		return nil
	},
}

var appsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		apps, err := apiClient.ListApps()
		if err != nil {
			return err
		}
		if len(apps) == 0 {
			fmt.Println("no apps yet")
			return nil
		}
		fmt.Printf("%-24s  %-10s  %s\n", "NAME", "STATUS", "ID")
		for _, a := range apps {
			fmt.Printf("%-24s  %-10s  %s\n", a.Name, a.Status, a.ID)
		}
		return nil
	},
}

func init() {
	appsCmd.AddCommand(appsCreateCmd, appsListCmd)
}
