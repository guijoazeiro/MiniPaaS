package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/guijoazeiro/MiniPaaS/cli/internal/api"
	cfgpkg "github.com/guijoazeiro/MiniPaaS/cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate and store the API token locally",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		defaultHost := apiClient.Host()
		fmt.Fprintf(os.Stderr, "host [%s]: ", defaultHost)
		host, _ := reader.ReadString('\n')
		host = strings.TrimSpace(host)
		if host == "" {
			host = defaultHost
		}

		fmt.Fprint(os.Stderr, "username: ")
		user, _ := reader.ReadString('\n')
		user = strings.TrimSpace(user)

		fmt.Fprint(os.Stderr, "password: ")
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}

		apiClient = api.New(host, "")
		resp, err := apiClient.Login(user, string(pw))
		if err != nil {
			return err
		}
		if err := cfgpkg.Save(&cfgpkg.Config{Host: host, Token: resp.Token}); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Println("logged in as", user)
		return nil
	},
}
