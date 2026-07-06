// Unit tests for the router's model-detail sub-route parsing (issue #51).
//
// The detail page lives at #/models/{name} — a deep-linkable, bookmarkable,
// reload-surviving sub-route of the flat "models" view, not an eighth top-level
// view. parse() is the pure hash→state function; it takes the raw hash string
// (defaulting to location.hash in the browser) so the sub-route grammar is
// unit-testable here without a jsdom or a real Location.
//
// Run under Node's zero-dependency built-in test runner via the runner in
// internal/web/js_test.go.

import { test } from "node:test";
import assert from "node:assert/strict";

import { parse, hashFor } from "../assets/js/router.js";

test("a bare view hash parses to that view with no detail", () => {
  const { view, detail } = parse("#/models");
  assert.equal(view, "models");
  assert.equal(detail, null);
});

test("#/models/{name} parses to the models view scoped to that model name", () => {
  const { view, detail } = parse("#/models/Post");
  assert.equal(view, "models");
  assert.equal(detail, "Post");
});

test("a model detail hash carrying query params keeps both the name and the params", () => {
  const { view, detail, params } = parse("#/models/Post?path=/tmp/app");
  assert.equal(view, "models");
  assert.equal(detail, "Post");
  assert.equal(params.get("path"), "/tmp/app");
});

test("a percent-encoded model name is decoded back to its literal form", () => {
  // Model names are class names (no slashes), but a defensive decode keeps a
  // name with reserved chars round-tripping through the URL intact.
  const { detail } = parse("#/models/My%20Model");
  assert.equal(detail, "My Model");
});

test("an unknown top-level view still falls back to overview with no detail", () => {
  const { view, detail } = parse("#/banana/Post");
  assert.equal(view, "overview");
  assert.equal(detail, null);
});

test("hashFor builds a bare view hash when no detail is given", () => {
  assert.equal(hashFor("models", null, new URLSearchParams()), "#/models");
});

test("hashFor builds a model-detail hash and round-trips through parse", () => {
  const params = new URLSearchParams({ path: "/tmp/app" });
  const hash = hashFor("models", "Post", params);
  assert.equal(hash, "#/models/Post?path=%2Ftmp%2Fapp");

  const back = parse(hash);
  assert.equal(back.view, "models");
  assert.equal(back.detail, "Post");
  assert.equal(back.params.get("path"), "/tmp/app");
});

test("hashFor percent-encodes a detail name so it round-trips intact", () => {
  const hash = hashFor("models", "My Model", new URLSearchParams());
  assert.equal(hash, "#/models/My%20Model");
  assert.equal(parse(hash).detail, "My Model");
});
