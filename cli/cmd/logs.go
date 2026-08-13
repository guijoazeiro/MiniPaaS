package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

var (
	logsFollow     bool
	logsTail       string
	logsDeployment string
)

var logsCmd = &cobra.Command{
	Use:   "logs <app>",
	Short: "Stream logs of the active container for an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := args[0]
		if logsDeployment != "" {
			items, err := apiClient.ListDeploymentLogs(app, logsDeployment, 0, 1000)
			if err != nil {
				return err
			}
			for _, item := range items {
				fmt.Fprintf(os.Stdout, "[%s] %s\n", item.Stage, item.Message)
			}
			return nil
		}
		wsURL, err := buildWSURL(apiClient.Host(), app, logsTail, logsFollow)
		if err != nil {
			return err
		}

		hdr := http.Header{}
		if stored != nil && stored.Token != "" {
			hdr.Set("Authorization", "Bearer "+stored.Token)
		}

		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, hdr)
		if err != nil {
			if resp != nil {
				return fmt.Errorf("dial ws (%s): %w", resp.Status, err)
			}
			return fmt.Errorf("dial ws: %w", err)
		}
		defer conn.Close()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		done := make(chan struct{})
		go func() {
			<-sigCh
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(2*time.Second))
			close(done)
			_ = conn.Close()
		}()

		type frame struct {
			TS     time.Time `json:"ts"`
			Stream string    `json:"stream"`
			Line   string    `json:"line"`
		}
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				select {
				case <-done:
					return nil
				default:
				}
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					return nil
				}
				return err
			}
			var f frame
			if err := json.Unmarshal(msg, &f); err != nil {
				fmt.Fprintln(os.Stderr, string(msg))
				continue
			}
			w := os.Stdout
			if f.Stream == "stderr" {
				w = os.Stderr
			}
			fmt.Fprintln(w, f.Line)
		}
	},
}

func buildWSURL(host, app, tail string, follow bool) (string, error) {
	u, err := url.Parse(host)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = "/apps/" + app + "/logs"
	q := u.Query()
	if follow {
		q.Set("follow", "true")
	}
	if tail != "" {
		q.Set("tail", tail)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "stream continuously until Ctrl+C")
	logsCmd.Flags().StringVar(&logsTail, "tail", "100", "number of trailing lines to fetch (or 'all')")
	logsCmd.Flags().StringVar(&logsDeployment, "deployment", "", "show persisted build logs for a deployment ID")
}
