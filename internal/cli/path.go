package cli

import "github.com/spf13/cobra"

// defaultProjectPath is used by every view command when no path argument is
// given, matching analyze's long-standing default of the current directory.
const defaultProjectPath = "."

// projectPathArg extracts the optional [path] positional argument shared by
// every view command (analyze, routes, models, er, openapi, findings),
// defaulting to the current directory when omitted.
func projectPathArg(_ *cobra.Command, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return defaultProjectPath
}
