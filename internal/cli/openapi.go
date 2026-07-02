package cli

// unlaravel openapi: the OpenAPI-spec view command (ADR 0008). Its default
// output already is JSON, so unlike the other view commands it has no --json
// flag — adding one would be a no-op that only invites confusion about which
// mode is "the real" output.

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/engine"
	"github.com/Mawsis/Un-La-ravel/internal/render/openapi"
)

func addOpenAPICommand() {
	openapiCmd := &cobra.Command{
		Use:   "openapi [path]",
		Short: "Print the generated OpenAPI 3 specification",
		Long: `Print the OpenAPI 3.0.3 specification generated from the project's
routes, controllers, and FormRequest validation rules.

The output is always JSON — there is no --json flag for this command, since
the OpenAPI document already is the JSON artifact. Pipe it to a file and open
it in Swagger UI or editor.swagger.io:

  unlaravel openapi ./app > openapi.json`,
		Args: cobra.MaximumNArgs(1),
		RunE: runOpenAPI,
	}
	rootCmd.AddCommand(openapiCmd)
}

func runOpenAPI(cmd *cobra.Command, args []string) error {
	projectPath := projectPathArg(cmd, args)
	out := cmd.OutOrStdout()

	pm, err := engine.Analyze(projectPath)
	if err != nil {
		return err
	}

	data, err := openapi.Render(pm)
	if err != nil {
		return fmt.Errorf("failed to render OpenAPI spec: %w", err)
	}

	fmt.Fprintln(out, string(data))
	return nil
}
