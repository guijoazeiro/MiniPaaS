package cmd

import (
	"fmt"

	"github.com/guijoazeiro/MiniPaaS/cli/internal/api"

	"github.com/spf13/cobra"
)

var gitRepository, gitBranch, gitContext, gitDockerfile string

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
		fmt.Printf("%-38s  %-11s  %-28s  %-6s  %s\n", "ID", "STATUS", "SOURCE", "PORT", "DURATION")
		for _, d := range deps {
			source := "tar"
			if d.Repository != "" {
				source = d.Repository + "@" + d.Branch
				if d.CommitSHA != "" {
					sha := d.CommitSHA
					if len(sha) > 8 {
						sha = sha[:8]
					}
					source += "#" + sha
				}
			}
			port := "-"
			if d.Port > 0 {
				port = fmt.Sprintf("%d", d.Port)
			}
			dur := "-"
			if d.DurationMs > 0 {
				dur = fmt.Sprintf("%dms", d.DurationMs)
			}
			fmt.Printf("%-38s  %-11s  %-28s  %-6s  %s\n", d.ID, d.Status, source, port, dur)
		}
		return nil
	},
}

var appsConnectGitHubCmd = &cobra.Command{
	Use: "connect-github <app>", Short: "Connect a public GitHub repository to an app", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if gitRepository == "" {
			return fmt.Errorf("--repo is required")
		}
		source, err := apiClient.ConfigureGitSource(args[0], api.GitSource{Repository: gitRepository, Branch: gitBranch, BuildContext: gitContext, DockerfilePath: gitDockerfile})
		if err != nil {
			return err
		}
		fmt.Printf("connected %s to %s@%s\n", args[0], source.Repository, source.Branch)
		fmt.Printf("  context: %s\n  dockerfile: %s\n", source.BuildContext, source.DockerfilePath)
		return nil
	},
}

var appsGitSourceCmd = &cobra.Command{
	Use: "git-source <app>", Short: "Show the connected GitHub repository", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source, err := apiClient.GetGitSource(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("%s@%s\n  context: %s\n  dockerfile: %s\n", source.Repository, source.Branch, source.BuildContext, source.DockerfilePath)
		return nil
	},
}

var appsDisconnectGitHubCmd = &cobra.Command{
	Use: "disconnect-github <app>", Short: "Disconnect the GitHub repository from an app", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := apiClient.DeleteGitSource(args[0]); err != nil {
			return err
		}
		fmt.Printf("disconnected GitHub repository from %s\n", args[0])
		return nil
	},
}

func init() {
	appsConnectGitHubCmd.Flags().StringVar(&gitRepository, "repo", "", "GitHub repository (owner/repository or URL)")
	appsConnectGitHubCmd.Flags().StringVar(&gitBranch, "branch", "main", "default branch")
	appsConnectGitHubCmd.Flags().StringVar(&gitContext, "context", ".", "build context relative to the repository root")
	appsConnectGitHubCmd.Flags().StringVar(&gitDockerfile, "dockerfile", "Dockerfile", "Dockerfile path relative to the build context")
	appsCmd.AddCommand(appsCreateCmd, appsListCmd, appsInfoCmd, appsConnectGitHubCmd, appsGitSourceCmd, appsDisconnectGitHubCmd)
}
