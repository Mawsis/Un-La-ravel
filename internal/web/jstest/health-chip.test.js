// Unit tests for the persistent sidebar health chip (issue #23).
//
// Run with the zero-dependency Node built-in test runner (no build step, no
// node_modules), same as chip.test.js / verdict.test.js:
//
//   node --test 'internal/web/jstest/**/*.test.js'
//
// The chip is the always-in-view summary of the same server-computed findings
// the Overview verdict itemizes (model.findings, contract 1.6.0): "clean" when
// the findings array is empty, or one summarized issue count otherwise. Like
// healthVerdict, it READS the server's findings — it never recomputes from raw
// dead_routes/disagreements/models. The state computation is pure (no DOM), so
// it is tested directly here.

import { test } from "node:test";
import assert from "node:assert/strict";

import { healthChip } from "../assets/js/verdict.js";

test("a clean project (empty findings) yields a clean chip with a zero count", () => {
  const chip = healthChip({ findings: [] });
  assert.equal(chip.clean, true);
  assert.equal(chip.count, 0);
  assert.equal(chip.label, "clean");
});

test("server findings sum into one summarized issue count", () => {
  const chip = healthChip({
    findings: [
      { kind: "dead_routes", count: 2, label: "2 dead routes", view: "findings" },
      { kind: "disagreements", count: 3, label: "3 disagreements", view: "findings" },
      { kind: "unguarded", count: 1, label: "1 unguarded model", view: "findings" },
    ],
  });
  assert.equal(chip.clean, false);
  assert.equal(chip.count, 6);
  assert.equal(chip.label, "6 issues");
});

test("a single issue reads in the singular", () => {
  const chip = healthChip({
    findings: [{ kind: "dead_routes", count: 1, label: "1 dead route", view: "findings" }],
  });
  assert.equal(chip.clean, false);
  assert.equal(chip.label, "1 issue");
});

test("a model with no findings key at all is treated as clean", () => {
  const chip = healthChip({});
  assert.equal(chip.clean, true);
  assert.equal(chip.count, 0);
  assert.equal(chip.label, "clean");
});

test("the chip READS findings and does not recompute from raw counts", () => {
  // Same authority rule as healthVerdict: a raw model that disagrees with its
  // (empty) findings follows the findings — the server is authoritative.
  const chip = healthChip({
    findings: [],
    dead_routes: [{ method: "GET", uri: "/x" }],
    models: [{ name: "Open", guarded: [] }],
  });
  assert.equal(chip.clean, true);
  assert.equal(chip.count, 0);
});
