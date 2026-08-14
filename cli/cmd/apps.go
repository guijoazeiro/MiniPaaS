package cmd

import (
	"fmt"
	"strconv"

	"github.com/guijoazeiro/MiniPaaS/cli/internal/api"

	"github.com/spf13/cobra"
)

var gitRepository, gitBranch, gitContext, gitDockerfile string
var gitInstallationID, gitRepositoryID int64
var deleteAppConfirmed bool

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

var appsDeleteCmd = &cobra.Command{
	Use: "delete <name>", Short: "Delete an app and its deployment history", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !deleteAppConfirmed {
			return fmt.Errorf("deleting an app is permanent; re-run with --yes to confirm")
		}
		if err := apiClient.DeleteApp(args[0]); err != nil {
			return err
		}
		fmt.Printf("deleted: %s\n", args[0])
		return nil
	},
}

var appsRetryCmd = &cobra.Command{
	Use: "retry <app> <deployment-id>", Short: "Retry a failed Git deployment", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dep, err := apiClient.RetryDeployment(args[0], args[1])
		if err != nil { return err }
		fmt.Printf("retry started: %s  status=%s  attempt=%d\n", dep.ID, dep.Status, dep.Attempt)
		return nil
	},
}

var appsCancelCmd = &cobra.Command{
	Use: "cancel <app> <deployment-id>", Short: "Cancel a pending or building deployment", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dep, err := apiClient.CancelDeployment(args[0], args[1])
		if err != nil { return err }
		fmt.Printf("cancel requested: %s  status=%s\n", dep.ID, dep.Status)
		return nil
	},
}

var appsConnectGitHubCmd = &cobra.Command{
	Use: "connect-github <app>", Short: "Connect a public or GitHub App repository to an app", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		private := gitInstallationID > 0 || gitRepositoryID > 0
		if private && (gitInstallationID <= 0 || gitRepositoryID <= 0) {
			return fmt.Errorf("--installation and --repository-id must be used together")
		}
		if !private && gitRepository == "" {
			return fmt.Errorf("--repo is required")
		}
		var source *api.GitSource
		var err error
		if private {
			source, err = apiClient.ConfigureGitHubAppSource(args[0], gitInstallationID, gitRepositoryID, gitBranch, gitContext, gitDockerfile)
		} else {
			source, err = apiClient.ConfigureGitSource(args[0], api.GitSource{Repository: gitRepository, Branch: gitBranch, BuildContext: gitContext, DockerfilePath: gitDockerfile})
		}
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
		fmt.Printf("%s@%s\n  access: %s\n  auto-deploy: %t\n  context: %s\n  dockerfile: %s\n", source.Repository, source.Branch, source.AccessMode, source.AutoDeploy, source.BuildContext, source.DockerfilePath)
		return nil
	},
}

var appsGitHubInstallationsCmd = &cobra.Command{
	Use: "github-installations", Short: "List GitHub App installations available to MiniPaaS",
	RunE: func(cmd *cobra.Command, args []string) error {
		installations, err := apiClient.ListGitHubInstallations()
		if err != nil {
			return err
		}
		if len(installations) == 0 {
			fmt.Println("no GitHub App installations; connect one from the dashboard first")
			return nil
		}
		fmt.Printf("%-14s  %-28s  %-14s  %s\n", "INSTALLATION", "ACCOUNT", "TYPE", "ACCESS")
		for _, installation := range installations {
			fmt.Printf("%-14d  %-28s  %-14s  %s\n", installation.InstallationID, installation.AccountLogin, installation.AccountType, installation.RepositorySelection)
		}
		return nil
	},
}

var appsGitHubRepositoriesCmd = &cobra.Command{
	Use: "github-repositories <installation-id>", Short: "List repositories available to a GitHub App installation", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		installationID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || installationID <= 0 {
			return fmt.Errorf("installation-id must be a positive integer")
		}
		repositories, err := apiClient.ListGitHubRepositories(installationID)
		if err != nil {
			return err
		}
		fmt.Printf("%-14s  %-42s  %-9s  %s\n", "REPOSITORY", "NAME", "VISIBILITY", "DEFAULT BRANCH")
		for _, repository := range repositories {
			visibility := "public"
			if repository.Private {
				visibility = "private"
			}
			fmt.Printf("%-14d  %-42s  %-9s  %s\n", repository.ID, repository.FullName, visibility, repository.DefaultBranch)
		}
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

var appsAutoDeployCmd = &cobra.Command{
	Use: "auto-deploy <app> <on|off>", Short: "Enable or disable GitHub push deployments", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var enabled bool
		switch args[1] {
		case "on":
			enabled = true
		case "off":
			enabled = false
		default:
			return fmt.Errorf("state must be on or off")
		}
		source, err := apiClient.SetGitAutoDeploy(args[0], enabled)
		if err != nil {
			return err
		}
		fmt.Printf("auto-deploy %s for %s@%s\n", args[1], source.Repository, source.Branch)
		return nil
	},
}

func init() {
	appsDeleteCmd.Flags().BoolVarP(&deleteAppConfirmed, "yes", "y", false, "confirm permanent deletion")
	appsConnectGitHubCmd.Flags().StringVar(&gitRepository, "repo", "", "GitHub repository (owner/repository or URL)")
	appsConnectGitHubCmd.Flags().StringVar(&gitBranch, "branch", "main", "default branch")
	appsConnectGitHubCmd.Flags().StringVar(&gitContext, "context", ".", "build context relative to the repository root")
	appsConnectGitHubCmd.Flags().StringVar(&gitDockerfile, "dockerfile", "Dockerfile", "Dockerfile path relative to the build context")
	appsConnectGitHubCmd.Flags().Int64Var(&gitInstallationID, "installation", 0, "GitHub App installation ID for a private repository")
	appsConnectGitHubCmd.Flags().Int64Var(&gitRepositoryID, "repository-id", 0, "GitHub repository ID exposed to the installation")
	appsCmd.AddCommand(appsCreateCmd, appsListCmd, appsInfoCmd, appsDeleteCmd, appsRetryCmd, appsCancelCmd, appsConnectGitHubCmd, appsGitSourceCmd, appsDisconnectGitHubCmd, appsGitHubInstallationsCmd, appsGitHubRepositoriesCmd, appsAutoDeployCmd)
}
