package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/guijoazeiro/MiniPaaS/cli/internal/tarball"
	"github.com/spf13/cobra"
)

var (
	deployApp string
	deployPoll bool
)

var deployCmd = &cobra.Command{
	Use:   "deploy [path]",
	Short: "Deploy a directory as a new version of an app",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if deployApp == "" {
			return fmt.Errorf("--app is required")
		}
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("path %q: %w", path, err)
		}

		fmt.Fprintf(os.Stderr, "packing %s...\n", path)
		pr, pw := io.Pipe()
		go func() {
			err := tarball.Pack(path, pw)
			_ = pw.CloseWithError(err)
		}()

		fmt.Fprintf(os.Stderr, "uploading to %s...\n", host)
		dep, err := apiClient.Deploy(deployApp, pr)
		if err != nil {
			return err
		}
		fmt.Printf("deployment %s  status=%s\n", dep.ID, dep.Status)

		if !deployPoll {
			fmt.Println("(pass --wait to poll until running or failed)")
			return nil
		}
		return waitForDeployment(deployApp, dep.ID)
	},
}

func waitForDeployment(app, id string) error {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	deadline := time.After(10 * time.Minute)
	for {
		select {
		case <-tick.C:
			d, err := apiClient.GetDeployment(app, id)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "  status=%s\n", d.Status)
			switch d.Status {
			case "running":
				fmt.Printf("running on host port %d\n", d.Port)
				return nil
			case "failed":
				return fmt.Errorf("deployment failed")
			}
		case <-deadline:
			return fmt.Errorf("timeout waiting for deployment")
		}
	}
}

func init() {
	deployCmd.Flags().StringVar(&deployApp, "app", "", "target app name (required)")
	deployCmd.Flags().BoolVar(&deployPoll, "wait", false, "poll until running or failed")
}
