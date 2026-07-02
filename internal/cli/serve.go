package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/web"
)

// portFlag is the name of the --port flag on `unlaravel serve`.
const portFlag = "port"

// addServeCommand adds the `serve` subcommand to the root command. It follows
// the same pattern as addAnalyzeCommand: build the cobra.Command, attach its
// flags, and register it — so `unlaravel serve` sits next to `unlaravel
// analyze` in the tree.
func addServeCommand() {
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the interactive analysis dashboard",
		Long: `Serve the Un(la)ravel dashboard over HTTP on localhost.

The serve command starts a local web server hosting an embedded single-page
dashboard and a JSON API. The API is JUST ANOTHER CONSUMER of the same analysis
engine the CLI uses (ADR 0001/0004: one Project Model, many renderers): each
request runs the identical pipeline and returns the identical unlaravel.json
contract, alongside the Mermaid ER diagram and OpenAPI 3 document.

Point the dashboard (or the API) at any local Laravel project:

  GET /api/analyze?path=<local-path>  -> { model, mermaid, openapi }
  GET /api/er?path=<local-path>       -> { mermaid }
  GET /api/openapi?path=<local-path>  -> OpenAPI 3 document

The server binds to localhost only; it is a local developer tool, not a public
service.`,
		Args: cobra.NoArgs,
		RunE: runServe,
	}

	serveCmd.Flags().IntP(portFlag, "p", web.DefaultPort, "Port to serve the dashboard on (localhost only)")

	rootCmd.AddCommand(serveCmd)
}

// runServe starts the dashboard server. It is PURE PRESENTATION: it reads the
// --port flag, prints the URL the developer should open, and blocks in
// web.Server.Start. All server behavior (routing, embedded UI, JSON API) lives
// in internal/web; the CLI only wires the flag and reports the address.
func runServe(cmd *cobra.Command, _ []string) error {
	port, err := cmd.Flags().GetInt(portFlag)
	if err != nil {
		return fmt.Errorf("failed to read --%s flag: %w", portFlag, err)
	}

	server, err := web.NewServer(port)
	if err != nil {
		return fmt.Errorf("failed to start dashboard: %w", err)
	}

	cyan := color.New(color.FgCyan)
	cyan.Printf("→ http://localhost:%d\n", port)

	// Start blocks until the server stops. A clean shutdown returns nil; any
	// other failure (e.g. the port is already in use) is surfaced to the caller,
	// which sets the process exit code.
	return server.Start()
}
