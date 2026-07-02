package cli

// unlaravel routes: the route view command (ADR 0008). Piped output is
// byte-identical to routemap.Render (the golden-pinned contract); a TTY gets
// a lipgloss-styled table; --json emits a routesJSON projection. This is the
// canonical shape every other TTY/pipe/--json view command in this package
// follows.
//
// The dead-route identity match is the one piece of logic reused rather than
// reimplemented (routemap.IsDeadRouteSet/IsDeadRoute): it's a
// correctness-sensitive algorithm, not a presentation detail, so it has
// exactly one implementation even though this file's table layout is
// otherwise independent of routemap's plain-text layout.

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Mawsis/Un-La-ravel/internal/engine"
	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/render/routemap"
)

func addRoutesCommand() {
	routesCmd := &cobra.Command{
		Use:   "routes [path]",
		Short: "List routes, with dead routes flagged",
		Long: `List every Route resolved from the project's route files, with method,
URI, controller@action, and middleware. A Route whose Controller/Action edge
does not resolve is flagged as a Dead Route (ADR 0006).

Piped output is byte-identical to the underlying route-map renderer, safe to
script against (e.g. "unlaravel routes ./app | grep POST"). --json emits the
routes and dead_routes arrays from the unlaravel.json contract.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runRoutes,
	}
	routesCmd.Flags().Bool("json", false, "Emit routes and dead_routes as JSON")
	rootCmd.AddCommand(routesCmd)
}

func runRoutes(cmd *cobra.Command, args []string) error {
	asJSON, _ := cmd.Flags().GetBool("json")
	projectPath := projectPathArg(cmd, args)
	out := cmd.OutOrStdout()

	pm, err := engine.Analyze(projectPath)
	if err != nil {
		return err
	}

	if asJSON {
		return emitJSON(out, routesJSON{
			SchemaVersion: pm.SchemaVersion,
			Routes:        pm.Routes,
			DeadRoutes:    pm.DeadRoutes,
		})
	}

	if !isTTY(out) {
		fmt.Fprint(out, routemap.Render(pm))
		return nil
	}

	printRoutesTable(out, pm)
	return nil
}

// printRoutesTable renders a styled route table for an interactive terminal.
// It is never golden-pinned (ADR 0008) — styling varies with terminal width
// and color profile — but it presents the same data as the plain renderer.
func printRoutesTable(out io.Writer, pm *model.ProjectModel) {
	deadSet := routemap.IsDeadRouteSet(pm.DeadRoutes)

	if len(pm.Routes) == 0 {
		fmt.Fprintln(out, styleDim.Render("No routes found."))
		return
	}

	widths := routeColumnWidths(pm.Routes)
	fmt.Fprintln(out, styleDim.Render(padRight("METHOD", widths.method)+"  "+
		padRight("URI", widths.uri)+"  "+
		padRight("CONTROLLER@ACTION", widths.action)+"  MIDDLEWARE"))

	for _, r := range pm.Routes {
		action := valueOrDash(r.Controller) + "@" + valueOrDash(r.Action)
		middleware := "-"
		if len(r.Middleware) > 0 {
			middleware = strings.Join(r.Middleware, ", ")
		}

		method := methodStyle(r.Method).Render(padRight(r.Method, widths.method))
		line := fmt.Sprintf("%s  %s  %s  %s",
			method, padRight(r.URI, widths.uri), padRight(action, widths.action), middleware)

		if routemap.IsDeadRoute(deadSet, r) {
			line += styleDanger.Render("  DEAD")
		}
		fmt.Fprintln(out, line)
	}

	fmt.Fprintf(out, "\n%d route(s)", len(pm.Routes))
	if len(pm.DeadRoutes) > 0 {
		fmt.Fprint(out, styleDanger.Render(fmt.Sprintf(", %d dead", len(pm.DeadRoutes))))
	}
	fmt.Fprintln(out)
}

// routeWidths carries the padded display width of each styled-table column,
// measured in runes throughout (see padRight) so multi-byte URIs, controller
// names, or actions don't skew alignment — matching routemap's own
// byte/rune discipline (internal/render/routemap/routemap.go's runeLen).
type routeWidths struct {
	method, uri, action int
}

func routeColumnWidths(routes []model.Route) routeWidths {
	w := routeWidths{
		method: runeLen("METHOD"),
		uri:    runeLen("URI"),
		action: runeLen("CONTROLLER@ACTION"),
	}
	for _, r := range routes {
		w.method = maxInt(w.method, runeLen(r.Method))
		w.uri = maxInt(w.uri, runeLen(r.URI))
		action := valueOrDash(r.Controller) + "@" + valueOrDash(r.Action)
		w.action = maxInt(w.action, runeLen(action))
	}
	return w
}

func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// padRight left-aligns s in a field of the given rune width, matching
// routemap's padRight exactly (width and content are both measured in runes,
// never bytes) so styled output aligns the same way plain output does for
// any non-ASCII route content.
func padRight(s string, width int) string {
	pad := width - runeLen(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func runeLen(s string) int {
	return len([]rune(s))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
