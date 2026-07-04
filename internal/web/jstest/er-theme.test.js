// Unit tests for the interim Mermaid ER theme mapping (issue #25).
//
// erThemeVariables is a pure function from the design tokens to Mermaid
// themeVariables, so the mapping — WHICH token paints WHICH part of the
// diagram — is testable with no DOM and no Mermaid. The browser side (er.js)
// only contributes the readToken closure over getComputedStyle.
//
//   node --test 'internal/web/jstest/**/*.test.js'

import { test } from "node:test";
import assert from "node:assert/strict";

import { erThemeVariables } from "../assets/js/views/er-theme.js";

// A fake token reader with the real token names and recognizable values, so a
// wrong-token regression (e.g. painting text with a surface color) is visible
// in the assertion diff.
const tokens = {
  "--surface-elevated": "#171717",
  "--surface-raised": "#262626",
  "--border-strong": "#525252",
  "--text": "#fafafa",
  "--text-dim": "#a3a3a3",
  "--mono": '"JetBrains Mono", monospace',
};
const readToken = (name) => tokens[name] ?? "";

test("maps the design tokens onto the parts of the ER diagram", () => {
  const vars = erThemeVariables(readToken);

  // Diagram canvas matches the #er-diagram container surface.
  assert.equal(vars.background, "#171717");
  // Entity boxes: raised surface with the strong border, readable text.
  assert.equal(vars.primaryColor, "#262626");
  assert.equal(vars.primaryBorderColor, "#525252");
  assert.equal(vars.primaryTextColor, "#fafafa");
  // Relationship lines read in dim text, not a leftover mermaid default.
  assert.equal(vars.lineColor, "#a3a3a3");
  // Attribute rows alternate between the two surface steps.
  assert.equal(vars.attributeBackgroundColorOdd, "#171717");
  assert.equal(vars.attributeBackgroundColorEven, "#262626");
  // Table/column names are code identifiers, so they render in the mono stack.
  assert.equal(vars.fontFamily, '"JetBrains Mono", monospace');
});

test("trims token values, since getComputedStyle returns leading whitespace", () => {
  const vars = erThemeVariables((name) => "  " + tokens[name] + " ");
  assert.equal(vars.primaryTextColor, "#fafafa");
});

test("base theme derivation runs in dark mode to match the dark-first tokens", () => {
  assert.equal(erThemeVariables(readToken).darkMode, true);
});
