// Unit tests for the empty-state sample-button logic (issue #29).
//
// Run with the zero-dependency Node built-in test runner (same convention as
// chip.test.js — no package.json, no node_modules, no build step):
//
//   node --test 'internal/web/jstest/**/*.test.js'
//
// samplePathFrom is the decision function behind the "try it on the sample
// project" button: given the /api/bootstrap response body it answers "is
// there a sample to offer, and where?" — null means the button stays hidden.
// It is a pure function of the response, so it is tested directly with no
// DOM; the reveal/click wiring in main.js is a thin consumer of this answer.

import { test } from "node:test";
import assert from "node:assert/strict";

import { samplePathFrom } from "../assets/js/views/hero.js";

test("a bootstrap response advertising a sample_path yields that path", () => {
  assert.equal(
    samplePathFrom({ default_path: "", sample_path: "/repo/testdata/fixture-app" }),
    "/repo/testdata/fixture-app"
  );
});

test("an empty sample_path means no sample: null hides the button", () => {
  assert.equal(samplePathFrom({ default_path: "", sample_path: "" }), null);
});

test("a bootstrap body without the field (older server) yields null", () => {
  assert.equal(samplePathFrom({ default_path: "" }), null);
});

test("a missing or malformed body yields null rather than throwing", () => {
  assert.equal(samplePathFrom(null), null);
  assert.equal(samplePathFrom(undefined), null);
  assert.equal(samplePathFrom({ sample_path: 42 }), null);
});
