// Unit tests for the Routes table row builder (issue #28: table polish +
// inline danger flags). routeRowHtml is the pure per-row template extracted
// from drawRouteRows so the row's observable markup — method color-class,
// mono-column classes, the DEAD danger flag and its cross-link to the
// dead_routes finding — is testable without a DOM, mirroring the
// chipTarget/entityChip split in chip.js.

import { test } from "node:test";
import assert from "node:assert/strict";

import { routeRowHtml } from "../assets/js/views/routes.js";

const route = {
  method: "GET",
  uri: "/users/{user}",
  controller: "UserController",
  action: "show",
  middleware: ["web", "auth"],
};

test("a route row color-codes its HTTP method via the m-<METHOD> class", () => {
  assert.match(routeRowHtml(route, false), /class="method m-GET"/);
  assert.match(
    routeRowHtml({ ...route, method: "DELETE" }, false),
    /class="method m-DELETE"/
  );
});

test("a live route row carries no danger flag", () => {
  const html = routeRowHtml(route, false);
  assert.doesNotMatch(html, /danger-flag/);
  assert.doesNotMatch(html, /class="dead"/);
});

test("a dead route row is flagged and cross-links to the dead_routes finding", () => {
  const html = routeRowHtml(route, true);
  assert.match(html, /<tr class="dead">/);
  // The DEAD marker is a chip (issue #24's cross-link language) painted with
  // the danger token, targeting the Findings view's dead_routes category.
  assert.match(html, /class="entity-chip danger-flag"/);
  assert.match(html, /data-view="findings"/);
  assert.match(html, /data-entity-id="dead_routes"/);
  assert.match(html, />DEAD<\/a>/);
});

test("a route row escapes attacker-shaped source strings", () => {
  const html = routeRowHtml({ ...route, uri: '<img src=x onerror=alert(1)>' }, false);
  assert.doesNotMatch(html, /<img/);
  assert.match(html, /&lt;img/);
});

test("a closure/view route renders a placeholder instead of a controller chip", () => {
  const html = routeRowHtml({ method: "GET", uri: "/health" }, false);
  assert.match(html, /\(closure \/ view route\)/);
  assert.doesNotMatch(html, /data-view="routes"/);
});
