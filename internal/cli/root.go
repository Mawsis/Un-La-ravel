package cli

import (
	"github.com/spf13/cobra"
)

// version is set via -ldflags "-X github.com/Mawsis/Un-La-ravel/internal/cli.version=..."
// at build/release time. It stays "dev" for local `go build`/`go run`.
var version = "dev"

// rootCmd represents the base command when called without any subcommands.
// In Cobra, commands are organized in a tree structure with a root command at
// the top.
var rootCmd = &cobra.Command{
	Use:     "unlaravel",
	Short:   "Un(la)ravel - Laravel project analysis tool",
	Version: version,
	Long: `Un(la)ravel - Laravel Project Analysis Tool

Un(la)ravel statically analyzes a Laravel project — no booting, no artisan,
no database connection — and builds a single Project Model of its schema,
Eloquent models, routes, controllers, and form requests.

  unlaravel analyze [path]    compact summary + findings digest
  unlaravel routes [path]     route table, dead routes flagged
  unlaravel models [path]     model cards: mass assignment, casts, indexes
  unlaravel er [path]         entity-relationship diagram (Mermaid)
  unlaravel findings [path]   disagreements + dead routes
  unlaravel openapi [path]    OpenAPI 3 specification (JSON)
  unlaravel serve [path]      interactive dashboard over the same model

Every view command accepts --json for a machine-readable projection (except
openapi, whose default output already is JSON), and adapts its plain output
to a pipe: piped output is byte-identical to the underlying renderer, so
"unlaravel routes ./app | grep POST" is stable to script against.

Use 'unlaravel help [command]' for more information about a command.`,
	// RunE is executed when the root command is called without subcommands.
	// The 'E' suffix means it returns an error (vs Run which doesn't).
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags
// appropriately. This is called by main.main(). It only needs to happen once
// to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// init registers every subcommand once, at package load.
func init() {
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	addAnalyzeCommand()
	addRoutesCommand()
	addModelsCommand()
	addERCommand()
	addOpenAPICommand()
	addFindingsCommand()
	addServeCommand()
}
