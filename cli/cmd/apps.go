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

var appsInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show app status, public URL, and recent deployments",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		app, err := apiClient.GetApp(name)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", app.Name)
		fmt.Printf("  status:  %s\n", app.Status)
		if app.ContainerState != "" {
			fmt.Printf("  container: %s\n", app.ContainerState)
		}
		if app.PublicURL != "" {
			fmt.Printf("  url:     %s\n", app.PublicURL)
		}
		fmt.Printf("  id:      %s\n", app.ID)

		deps, err := apiClient.ListDeployments(name)
		if err != nil {
			return err
		}
		if len(deps) == 0 {
			return nil
		}
		fmt.Printf("\nrecent deployments\n")
		fmt.Printf("%-38s  %-11s  %-6s  %s\n", "ID", "STATUS", "PORT", "DURATION")
		for _, d := range deps {
			port := "-"
			if d.Port > 0 {
				port = fmt.Sprintf("%d", d.Port)
			}
			dur := "-"
			if d.DurationMs > 0 {
				dur = fmt.Sprintf("%dms", d.DurationMs)
			}
			fmt.Printf("%-38s  %-11s  %-6s  %s\n", d.ID, d.Status, port, dur)
		}
		return nil
	},
}

func init() {
	appsCmd.AddCommand(appsCreateCmd, appsListCmd, appsInfoCmd)
}
