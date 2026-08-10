package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/guijoazeiro/MiniPaaS/cli/internal/api"
	"github.com/spf13/cobra"
)

var rollbackTo string

var rollbackCmd = &cobra.Command{
	Use:   "rollback <app>",
	Short: "Roll back to a previous deployment",
	Long: `Restarts the container from a previous deployment's image and repoints Caddy.
Without --to the command lists eligible deployments and prompts for a choice.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := args[0]

		target := rollbackTo
		if target == "" {
			chosen, err := pickDeployment(app)
			if err != nil {
				return err
			}
			target = chosen
		}

		dep, err := apiClient.Rollback(app, target)
		if err != nil {
			return err
		}
		fmt.Printf("rolled back to %s  status=%s  port=%d\n", dep.ID, dep.Status, dep.Port)
		return nil
	},
}

func pickDeployment(app string) (string, error) {
	deps, err := apiClient.ListDeployments(app)
	if err != nil {
		return "", err
	}
	eligible := []api.Deployment{}
	for _, d := range deps {
		if d.ImageTag == "" || d.Status == "running" || d.Status == "pending" || d.Status == "building" || d.Status == "failed" {
			continue
		}
		eligible = append(eligible, d)
	}
	if len(eligible) == 0 {
		return "", fmt.Errorf("no eligible deployments to roll back to")
	}

	fmt.Println("choose a deployment to roll back to:")
	for i, d := range eligible {
		fmt.Printf("  [%d] %s  status=%-11s  image=%s\n", i+1, d.ID, d.Status, d.ImageTag)
	}
	fmt.Print("selection [1]: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return eligible[0].ID, nil
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(eligible) {
		return "", fmt.Errorf("invalid selection %q", line)
	}
	return eligible[n-1].ID, nil
}

func init() {
	rollbackCmd.Flags().StringVar(&rollbackTo, "to", "", "deployment id to roll back to (skip the interactive picker)")
}
