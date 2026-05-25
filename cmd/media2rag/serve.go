package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	serveHost string
	servePort int
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		host := serveHost
		port := servePort
		if cfg != nil {
			if host == "localhost" && servePort == 8542 {
				host = cfg.Server.Host
				port = cfg.Server.Port
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "server not yet implemented (would start on %s:%d)\n", host, port)
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveHost, "host", "localhost", "server host")
	serveCmd.Flags().IntVar(&servePort, "port", 8542, "server port")
	rootCmd.AddCommand(serveCmd)
}
