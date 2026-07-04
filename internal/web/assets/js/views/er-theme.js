// Interim Mermaid ER theming (issue #25): map the design tokens onto Mermaid's
// "base" theme variables so the vendored Mermaid diagram renders in the same
// palette as the rest of the redesign, until the owned SVG renderer (issue
// #19) replaces it. Styling only — no engine or contract change.
//
// This is a pure function of a token reader so the mapping is unit-testable
// (jstest/er-theme.test.js) without a DOM; er.js supplies the real reader as a
// closure over getComputedStyle. tokens.css stays the single source of truth —
// no color literal appears here.

// erThemeVariables builds Mermaid themeVariables from the design tokens.
// readToken takes a CSS custom property name ("--text") and returns its value;
// values are trimmed because getComputedStyle returns leading whitespace.
export function erThemeVariables(readToken) {
  const token = (name) => String(readToken(name)).trim();
  return {
    // Derive unlisted colors dark-side-first, matching the dark-first tokens.
    darkMode: true,
    // Canvas matches the #er-diagram container surface, so the diagram sits
    // ON the panel instead of floating in a foreign background.
    background: token("--surface-elevated"),
    // Entity boxes: one surface step up, strong border, readable text.
    primaryColor: token("--surface-raised"),
    primaryBorderColor: token("--border-strong"),
    primaryTextColor: token("--text"),
    // Relationship lines and their cardinality labels.
    lineColor: token("--text-dim"),
    // Attribute rows alternate between the two surface steps.
    attributeBackgroundColorOdd: token("--surface-elevated"),
    attributeBackgroundColorEven: token("--surface-raised"),
    // Table/column names are code identifiers → mono stack, like entity chips.
    fontFamily: token("--mono"),
  };
}
