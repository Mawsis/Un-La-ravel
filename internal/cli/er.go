package cli

// unlaravel er: the ER-diagram view command (ADR 0008). The Mermaid source
// is not something lipgloss can meaningfully style — it's diagram markup
// meant for a renderer like mermaid.live or the web dashboard — so "styled"
// and "piped" modes are identical: er.Render(pm) verbatim, byte-equal to the
// golden-pinned contract in every case. --json wraps the same string with the
// schema version.

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/engine"
	"github.com/Mawsis/Un-La-ravel/internal/render/er"
)

func addERCommand() {
	erCmd := &cobra.Command{
		Use:   "er [path]",
		Short: "Print the entity-relationship diagram (Mermaid)",
		Long: `Print the Mermaid entity-relationship diagram: table/column structure
from migrations, plus relationship lines from Eloquent models.

Paste the output into mermaid.live, a GitHub Markdown file, or Obsidian to
render it. --json wraps the same Mermaid source with the schema version.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runER,
	}
	erCmd.Flags().Bool("json", false, "Emit the Mermaid source as JSON")
	rootCmd.AddCommand(erCmd)
}

func runER(cmd *cobra.Command, args []string) error {
	asJSON, _ := cmd.Flags().GetBool("json")
	projectPath := projectPathArg(cmd, args)
	out := cmd.OutOrStdout()

	pm, err := engine.Analyze(projectPath)
	if err != nil {
		return err
	}

	mermaid := er.Render(pm)

	if asJSON {
		return emitJSON(out, erJSON{
			SchemaVersion: pm.SchemaVersion,
			Mermaid:       mermaid,
		})
	}

	fmt.Fprint(out, mermaid)
	return nil
}
