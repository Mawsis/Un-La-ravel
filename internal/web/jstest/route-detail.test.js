// Unit tests for the Route detail page's compose logic (issue #68).
//
// The page (#/routes/{key}) shows ONE route's complete binding: method + URI,
// the resolved Controller + Action, the Middleware applied to it (linked to
// their nodes), its Auth state, and any bound FormRequest. composeRouteDetail
// is the pure function at its heart — route key + whole Project Model in,
// everything the page renders out — mirroring composeModelDetail so the two
// detail pages can't drift apart.
//
// The load-bearing cases pinned here are the ones the contract makes non-
// obvious: a route key must survive a URI containing slashes and braces,
// middleware names arrive PARAMETERIZED ("auth:sanctum") and must match their
// node on the base alias, and direct-vs-group application must stay two labeled
// relations rather than one merged list.

import { test } from "node:test";
import assert from "node:assert/strict";

import { composeRouteDetail, routeDetailHtml, routeKey } from "../assets/js/views/route-detail.js";

function projectModel(overrides) {
  return {
    routes: [],
    controllers: [],
    middlewares: [],
    form_requests: [],
    dead_routes: [],
    ...overrides,
  };
}

// A route as the contract actually serializes it (internal/model/route.go).
function route(overrides) {
  return {
    method: "GET",
    uri: "/posts",
    controller: "PostController",
    action: "index",
    middleware: [],
    auth: "unauthenticated",
    ...overrides,
  };
}

test("a route is addressed by a method+URI key that survives slashes and braces", () => {
  // The key goes in a URL path segment, so it must round-trip a URI that is
  // itself full of slashes and braces — the reason the key is not just the raw
  // URI. Two routes differing only by method must get distinct keys.
  const get = routeKey(route({ method: "GET", uri: "/admin/posts/{post}/comments" }));
  const post = routeKey(route({ method: "POST", uri: "/admin/posts/{post}/comments" }));

  assert.notEqual(get, post);
  assert.ok(!get.includes("/"), "the key must not contain a raw slash: " + get);
});

test("the key round-trips: composing by a route's own key finds that route", () => {
  const r = route({ method: "DELETE", uri: "/admin/posts/{post}" });
  const pm = projectModel({ routes: [route({ method: "GET", uri: "/posts" }), r] });

  const detail = composeRouteDetail(routeKey(r), pm);

  assert.equal(detail.found, true);
  assert.equal(detail.method, "DELETE");
  assert.equal(detail.uri, "/admin/posts/{post}");
});

test("a key survives the real link → router → compose round-trip", () => {
  // The end-to-end path a click actually takes: the key is encoded into an href
  // segment, the router percent-decodes exactly ONE layer (safeDecode), and the
  // result must still find the route. This is the integration the individual
  // key tests can't see — and where a double-encoding bug would surface.
  const r = route({ method: "GET", uri: "/admin/posts/{post}/comments" });
  const pm = projectModel({ routes: [r] });

  const hrefSegment = encodeURIComponent(routeKey(r)); // what the link emits
  const afterRouter = decodeURIComponent(hrefSegment); // what parse() yields

  assert.equal(afterRouter, routeKey(r));
  assert.equal(composeRouteDetail(afterRouter, pm).found, true);
  // And the segment stays legible rather than becoming percent-soup.
  assert.ok(!hrefSegment.includes("%25"), "key must not double-encode: " + hrefSegment);
});

test("an unknown key yields a found:false view rather than throwing", () => {
  const detail = composeRouteDetail("GET /nope", projectModel({}));

  assert.equal(detail.found, false);
  assert.deepEqual(detail.middleware, []);
});

test("the resolved controller FQN and action are shown, not the verbatim reference", () => {
  // Route.Controller is recorded VERBATIM (issue #63) and Route.FQN carries the
  // phase-two resolution. The page must lead with the FQN — showing only the
  // bare short name is what made sub-namespaced controllers look dead.
  const r = route({
    controller: "AdminDashboardController",
    fqn: "App\\Http\\Controllers\\Admin\\AdminDashboardController",
    action: "index",
  });
  const detail = composeRouteDetail(routeKey(r), projectModel({ routes: [r] }));

  assert.equal(detail.controller.fqn, "App\\Http\\Controllers\\Admin\\AdminDashboardController");
  assert.equal(detail.controller.action, "index");
  // The verbatim reference is kept too — it is what the source literally wrote.
  assert.equal(detail.controller.reference, "AdminDashboardController");
});

test("an unresolved controller is reported as such, and the route reads as dead", () => {
  // No FQN means phase two could not resolve it — the same state recorded as a
  // DeadRoute. The page must say so rather than showing a blank controller.
  const r = route({ controller: "GhostController", action: "index" });
  const pm = projectModel({
    routes: [r],
    dead_routes: [{ method: "GET", uri: "/posts", controller: "GhostController", action: "index" }],
  });

  const detail = composeRouteDetail(routeKey(r), pm);

  assert.equal(detail.controller.resolved, false);
  assert.equal(detail.dead, true);
});

test("the auth state comes from the contract, never re-classified in the browser", () => {
  // Route.Auth is computed server-side by the classifier (issue #50, ADR 0008).
  // Re-deriving it from the middleware list here would let the page and the
  // Auth view disagree.
  const r = route({ middleware: ["auth"], auth: "authenticated" });
  const detail = composeRouteDetail(routeKey(r), projectModel({ routes: [r] }));

  assert.equal(detail.auth, "authenticated");
});

test("middleware are linked to their nodes, matched on the base alias", () => {
  // Applied names arrive PARAMETERIZED — "auth:sanctum", "throttle:api" — but
  // the Middleware node's alias is the bare "auth"/"throttle". Matching on the
  // full string would link nothing on exactly the routes that matter.
  const r = route({ middleware: ["auth:sanctum", "throttle:api"] });
  const pm = projectModel({
    routes: [r],
    middlewares: [
      { alias: "auth", class: "App\\Http\\Middleware\\Authenticate", groups: [], origin: "app" },
      { alias: "throttle", class: "Illuminate\\Routing\\Middleware\\ThrottleRequests", groups: ["api"], origin: "app" },
    ],
  });

  const detail = composeRouteDetail(routeKey(r), pm);

  assert.deepEqual(
    detail.middleware.map((m) => ({ applied: m.applied, alias: m.alias, resolved: m.resolved })),
    [
      { applied: "auth:sanctum", alias: "auth", resolved: true },
      { applied: "throttle:api", alias: "throttle", resolved: true },
    ]
  );
  // The parameter is preserved — "auth:sanctum" says which guard, and dropping
  // it would lose the fact the route authenticates against sanctum.
  assert.equal(detail.middleware[0].parameter, "sanctum");
  assert.equal(detail.middleware[0].class, "App\\Http\\Middleware\\Authenticate");
});

test("a middleware with no matching node is kept, marked unresolved", () => {
  // An applied-but-undeclared name is real information (ADR 0002: report what
  // the source says). Dropping it would hide that the route applies something
  // the Kernel never declared.
  const r = route({ middleware: ["audit.log"] });
  const detail = composeRouteDetail(routeKey(r), projectModel({ routes: [r], middlewares: [] }));

  assert.equal(detail.middleware.length, 1);
  assert.equal(detail.middleware[0].resolved, false);
  assert.equal(detail.middleware[0].alias, "audit.log");
});

test("a middleware without a parameter carries an empty parameter, not undefined", () => {
  const r = route({ middleware: ["auth"] });
  const pm = projectModel({ routes: [r], middlewares: [{ alias: "auth", groups: [], origin: "app" }] });

  assert.equal(composeRouteDetail(routeKey(r), pm).middleware[0].parameter, "");
});

// --- direct vs. group application ------------------------------------------
//
// Route.Middleware is a FLAT list: the contract merges names applied directly at
// the route site with names inherited from an enclosing group, and records no
// provenance per name (see the field doc in internal/model/route.go). The split
// is therefore DERIVED here, from the middleware node's own Groups: a name whose
// node belongs to a group that is also applied on this route arrived via that
// group; everything else was applied directly.

test("middleware applied via a group are separated from those applied directly", () => {
  const r = route({ middleware: ["api", "throttle:api", "tenant"] });
  const pm = projectModel({
    routes: [r],
    middlewares: [
      // throttle belongs to the "api" group, which this route also applies.
      { alias: "throttle", groups: ["api"], origin: "app" },
      // tenant belongs to no group — it can only have been applied directly.
      { alias: "tenant", groups: [], origin: "app" },
    ],
  });

  const detail = composeRouteDetail(routeKey(r), pm);

  assert.deepEqual(detail.viaGroup.map((m) => m.alias), ["throttle"]);
  assert.equal(detail.viaGroup[0].group, "api");
  // "api" is in `direct` alongside "tenant": naming a group at the route site is
  // itself a direct application (see the dedicated test below). It is the
  // group's MEMBERS that arrive indirectly, not the act of applying the group.
  assert.deepEqual(detail.direct.map((m) => m.alias), ["api", "tenant"]);
});

test("the two relations are never merged — every applied name lands in exactly one", () => {
  // The acceptance criterion is that they stay distinct AND complete: a name
  // must not vanish between the two lists, nor be double-counted in both.
  const r = route({ middleware: ["web", "auth", "tenant"] });
  const pm = projectModel({
    routes: [r],
    middlewares: [
      { alias: "auth", groups: ["web"], origin: "app" },
      { alias: "tenant", groups: [], origin: "app" },
    ],
  });

  const detail = composeRouteDetail(routeKey(r), pm);
  const partitioned = [...detail.direct, ...detail.viaGroup].map((m) => m.applied).sort();

  assert.deepEqual(partitioned, ["auth", "tenant", "web"].sort());
  const aliases = new Set();
  [...detail.direct, ...detail.viaGroup].forEach((m) => {
    assert.ok(!aliases.has(m.applied), m.applied + " appears in both lists");
    aliases.add(m.applied);
  });
});

test("a group name applied on the route is itself shown as a direct application", () => {
  // Applying "web" IS a direct act at the route site, even though its effect is
  // to pull in a group. It must not be silently swallowed into viaGroup.
  const r = route({ middleware: ["web"] });
  const pm = projectModel({ routes: [r], middlewares: [{ alias: "auth", groups: ["web"], origin: "app" }] });

  const detail = composeRouteDetail(routeKey(r), pm);

  assert.deepEqual(detail.direct.map((m) => m.applied), ["web"]);
  assert.deepEqual(detail.viaGroup, []);
});

test("the bound FormRequest is resolved to its full object so its fields show", () => {
  const r = route({ method: "POST", form_request: "App\\Http\\Requests\\StorePostRequest" });
  const pm = projectModel({
    routes: [r],
    form_requests: [
      {
        name: "StorePostRequest",
        fqn: "App\\Http\\Requests\\StorePostRequest",
        fields: [{ name: "title", rules: ["required"] }],
      },
    ],
  });

  const detail = composeRouteDetail(routeKey(r), pm);

  assert.equal(detail.formRequest.name, "StorePostRequest");
  assert.deepEqual(detail.formRequest.fields.map((f) => f.name), ["title"]);
});

test("a route with no FormRequest carries null, not a fabricated empty one", () => {
  const r = route();
  const detail = composeRouteDetail(routeKey(r), projectModel({ routes: [r] }));
  assert.equal(detail.formRequest, null);
});

test("a form_request naming a class that was never extracted is reported, not dropped", () => {
  const r = route({ method: "POST", form_request: "App\\Http\\Requests\\GhostRequest" });
  const detail = composeRouteDetail(routeKey(r), projectModel({ routes: [r], form_requests: [] }));

  // The binding is real even though the class wasn't found — say so rather than
  // showing "no form request", which would be a different (false) claim.
  assert.equal(detail.formRequest.name, "GhostRequest");
  assert.equal(detail.formRequest.resolved, false);
});

test("middleware order follows the route's applied order (determinism)", () => {
  const r = route({ middleware: ["c", "a", "b"] });
  const pm = projectModel({ routes: [r], middlewares: [] });

  assert.deepEqual(
    composeRouteDetail(routeKey(r), pm).middleware.map((m) => m.alias),
    ["c", "a", "b"]
  );
});

// --- markup ----------------------------------------------------------------

test("the method badge carries the uppercase m-<METHOD> class the CSS keys on", () => {
  // The color rules are `.m-GET`, `.m-POST`, … (components.css). A lowercased
  // or bare `method` class matches NO rule, so the badge renders as unstyled
  // inline text — a failure no compose-level test can see, which is why the
  // class name is pinned here.
  const r = route({ method: "POST", uri: "/posts" });
  const html = routeDetailHtml(composeRouteDetail(routeKey(r), projectModel({ routes: [r] })));

  assert.match(html, /class="method m-POST"/);
});

test("an unresolved controller is flagged with a non-color channel, not colour alone", () => {
  // DESIGN.md §1 hard rule: the status dot is decorative (aria-hidden), so the
  // word "Unresolved" has to carry the meaning on its own.
  const r = route({ controller: "GhostController" });
  const html = routeDetailHtml(composeRouteDetail(routeKey(r), projectModel({ routes: [r] })));

  assert.match(html, /Unresolved/);
  assert.match(html, /aria-hidden="true"/);
});

test("the page escapes a URI and controller reference from parsed PHP source", () => {
  const r = route({ uri: "/<img src=x onerror=alert(1)>", controller: "<script>" });
  const html = routeDetailHtml(composeRouteDetail(routeKey(r), projectModel({ routes: [r] })));

  assert.doesNotMatch(html, /<img|<script>/);
  assert.match(html, /&lt;/);
});
