// Unit tests for the Controller detail page's compose logic (issue #69).
//
// The page (#/controllers/{fqn}) shows ONE controller's public actions and, per
// action, the Routes that dispatch to it. It benefits directly from the
// controller-FQN fix (#63): routes-per-action is only correct once
// sub-namespaced controllers resolve, so the tests below lean on Route.FQN
// rather than the verbatim Route.Controller reference.
//
// composeControllerDetail is pure (fqn + whole Project Model in, view-model
// out), the same split as composeModelDetail / composeRouteDetail.

import { test } from "node:test";
import assert from "node:assert/strict";

import { composeControllerDetail } from "../assets/js/views/controller-detail.js";

function projectModel(overrides) {
  return { controllers: [], routes: [], dead_routes: [], ...overrides };
}

const POST_FQN = "App\\Http\\Controllers\\PostController";
const ADMIN_FQN = "App\\Http\\Controllers\\Admin\\AdminDashboardController";

test("an unknown fqn yields a found:false view rather than throwing", () => {
  const detail = composeControllerDetail("App\\Nope", projectModel({}));

  assert.equal(detail.found, false);
  assert.deepEqual(detail.actions, []);
});

test("the controller's public actions are listed in source-declaration order", () => {
  const pm = projectModel({
    controllers: [{ name: "PostController", fqn: POST_FQN, actions: ["index", "show", "store"] }],
  });

  const detail = composeControllerDetail(POST_FQN, pm);

  assert.equal(detail.found, true);
  assert.equal(detail.name, "PostController");
  // Declaration order, NOT alphabetical — determinism is a hard convention and
  // the source order is the fact the contract preserves.
  assert.deepEqual(detail.actions.map((a) => a.name), ["index", "show", "store"]);
});

test("each action lists the routes that dispatch to it", () => {
  const pm = projectModel({
    controllers: [{ name: "PostController", fqn: POST_FQN, actions: ["index", "store"] }],
    routes: [
      { method: "GET", uri: "/posts", controller: "PostController", action: "index", fqn: POST_FQN, middleware: [] },
      { method: "POST", uri: "/posts", controller: "PostController", action: "store", fqn: POST_FQN, middleware: [] },
      { method: "GET", uri: "/feed", controller: "PostController", action: "index", fqn: POST_FQN, middleware: [] },
    ],
  });

  const byName = new Map(composeControllerDetail(POST_FQN, pm).actions.map((a) => [a.name, a]));

  assert.deepEqual(byName.get("index").routes.map((r) => r.uri), ["/posts", "/feed"]);
  assert.deepEqual(byName.get("store").routes.map((r) => r.uri), ["/posts"]);
});

test("routes are matched on the resolved FQN, not the verbatim reference", () => {
  // The point of #63: two controllers can share a short name across namespaces.
  // Matching on Route.Controller ("DashboardController") would attribute both
  // routes to whichever page you happened to open.
  const OTHER_FQN = "App\\Http\\Controllers\\Reports\\DashboardController";
  const pm = projectModel({
    controllers: [
      { name: "DashboardController", fqn: ADMIN_FQN, actions: ["index"] },
      { name: "DashboardController", fqn: OTHER_FQN, actions: ["index"] },
    ],
    routes: [
      { method: "GET", uri: "/admin", controller: "DashboardController", action: "index", fqn: ADMIN_FQN, middleware: [] },
      { method: "GET", uri: "/reports", controller: "DashboardController", action: "index", fqn: OTHER_FQN, middleware: [] },
    ],
  });

  const admin = composeControllerDetail(ADMIN_FQN, pm);

  assert.deepEqual(admin.actions[0].routes.map((r) => r.uri), ["/admin"]);
});

test("an action with no routes is still listed, marked as dispatched by none", () => {
  // A public method no route reaches is real, useful information (dead code, or
  // a method called internally). Hiding it would make the page a route list
  // rather than a controller page.
  const pm = projectModel({
    controllers: [{ name: "PostController", fqn: POST_FQN, actions: ["index", "orphan"] }],
    routes: [
      { method: "GET", uri: "/posts", controller: "PostController", action: "index", fqn: POST_FQN, middleware: [] },
    ],
  });

  const byName = new Map(composeControllerDetail(POST_FQN, pm).actions.map((a) => [a.name, a]));

  assert.deepEqual(byName.get("orphan").routes, []);
  assert.equal(byName.get("orphan").dispatched, false);
  assert.equal(byName.get("index").dispatched, true);
});

test("a route dispatching to a method the controller does not declare is surfaced", () => {
  // The route names an action absent from the class — a real mismatch (a
  // renamed method, a typo) that must not vanish just because no action row
  // claims it.
  const pm = projectModel({
    controllers: [{ name: "PostController", fqn: POST_FQN, actions: ["index"] }],
    routes: [
      { method: "GET", uri: "/posts", controller: "PostController", action: "index", fqn: POST_FQN, middleware: [] },
      { method: "GET", uri: "/gone", controller: "PostController", action: "vanished", fqn: POST_FQN, middleware: [] },
    ],
  });

  const detail = composeControllerDetail(POST_FQN, pm);

  assert.deepEqual(detail.undeclaredActions.map((a) => a.name), ["vanished"]);
  assert.deepEqual(detail.undeclaredActions[0].routes.map((r) => r.uri), ["/gone"]);
});

test("a controller with no actions at all composes cleanly", () => {
  // Base controllers (App\Http\Controllers\Controller) legitimately have none.
  const FQN = "App\\Http\\Controllers\\Controller";
  const pm = projectModel({ controllers: [{ name: "Controller", fqn: FQN, actions: [] }] });

  const detail = composeControllerDetail(FQN, pm);

  assert.equal(detail.found, true);
  assert.deepEqual(detail.actions, []);
  assert.deepEqual(detail.undeclaredActions, []);
});

test("each listed route carries the method and auth state the row needs", () => {
  const pm = projectModel({
    controllers: [{ name: "PostController", fqn: POST_FQN, actions: ["store"] }],
    routes: [
      {
        method: "POST",
        uri: "/posts",
        controller: "PostController",
        action: "store",
        fqn: POST_FQN,
        middleware: ["auth"],
        auth: "authenticated",
      },
    ],
  });

  const r = composeControllerDetail(POST_FQN, pm).actions[0].routes[0];

  assert.equal(r.method, "POST");
  assert.equal(r.auth, "authenticated");
  // A key so the row can link straight through to the Route detail page (#68).
  assert.ok(r.key, "each route needs a key to cross-link to its detail page");
});

test("the route count across all actions is exposed for the page header", () => {
  const pm = projectModel({
    controllers: [{ name: "PostController", fqn: POST_FQN, actions: ["index", "store"] }],
    routes: [
      { method: "GET", uri: "/posts", controller: "PostController", action: "index", fqn: POST_FQN, middleware: [] },
      { method: "POST", uri: "/posts", controller: "PostController", action: "store", fqn: POST_FQN, middleware: [] },
    ],
  });

  assert.equal(composeControllerDetail(POST_FQN, pm).routeCount, 2);
});

test("routes preserve their source order within an action (determinism)", () => {
  const pm = projectModel({
    controllers: [{ name: "PostController", fqn: POST_FQN, actions: ["index"] }],
    routes: [
      { method: "GET", uri: "/z", controller: "PostController", action: "index", fqn: POST_FQN, middleware: [] },
      { method: "GET", uri: "/a", controller: "PostController", action: "index", fqn: POST_FQN, middleware: [] },
      { method: "GET", uri: "/m", controller: "PostController", action: "index", fqn: POST_FQN, middleware: [] },
    ],
  });

  assert.deepEqual(
    composeControllerDetail(POST_FQN, pm).actions[0].routes.map((r) => r.uri),
    ["/z", "/a", "/m"]
  );
});

test("a controller can also be addressed by its short name when unambiguous", () => {
  // Deep links and chips may carry the short name. Resolving it is a
  // convenience, but it must NOT fire when the name is ambiguous across
  // namespaces — that is exactly the false attribution #63 fixed.
  const pm = projectModel({
    controllers: [{ name: "PostController", fqn: POST_FQN, actions: ["index"] }],
  });

  assert.equal(composeControllerDetail("PostController", pm).found, true);
});

test("an ambiguous short name does NOT resolve to an arbitrary controller", () => {
  const OTHER = "App\\Http\\Controllers\\Reports\\DashboardController";
  const pm = projectModel({
    controllers: [
      { name: "DashboardController", fqn: ADMIN_FQN, actions: [] },
      { name: "DashboardController", fqn: OTHER, actions: [] },
    ],
  });

  const detail = composeControllerDetail("DashboardController", pm);

  assert.equal(detail.found, false);
  assert.equal(detail.ambiguous, true);
  // Both candidates are named, so the page can offer the choice rather than
  // guessing — silently picking one is the failure mode #63 was about.
  assert.deepEqual(detail.candidates.sort(), [ADMIN_FQN, OTHER].sort());
});
