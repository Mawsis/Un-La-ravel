// Auth view (issue #50): auth coverage under the HEALTH nav group. It leads with
// a verdict sentence in the tool's voice ("3 of 6 routes have no auth middleware,
// 1 of them write") and then lists the routes reachable without authentication,
// grouped by their middleware stack.
//
// It READS the contract, it does not recompute it (ADR 0008): each route already
// carries the server-stamped `auth` field ("authenticated" / "unauthenticated" /
// "unknown"), computed once by the Go classifier in internal/findings. The view
// trusts that field — it never re-classifies middleware in the browser — so the
// dashboard and the CLI can never disagree about a route's auth state.
//
// Findings are facts, not alarms (design.md principle 4): the verdict copy stays
// specific and calm. Severity reads as a leading status dot plus spelled-out text
// (a write says "write"), never a bare hue or a side-stripe — the non-color
// channel ADR 0011 requires, so a red/green-blind reader gets the signal too.

import { $, escapeHtml } from "../dom.js";
import { hrefFor } from "../links.js";

// STACK_NONE_LABEL is the group heading for routes with no middleware at all —
// the most common unauthenticated case. A literal "(no middleware)" reads
// clearer than an empty heading.
export const STACK_NONE_LABEL = "(no middleware)";

// WRITE_METHODS is the set of mutating HTTP verbs, mirroring the server's
// isWriteMethod (internal/findings/items.go): an unauthenticated route with one
// of these is a blocker, any other verb is a warning.
const WRITE_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);

// isWriteMethod reports whether a verb mutates state, matching the server split
// so the browser's severity for a route agrees with the contract's finding kind.
// Case-insensitive, though the extractor uppercases.
export function isWriteMethod(method) {
  return WRITE_METHODS.has((method || "").toUpperCase());
}

// isUnauthenticated reads the contract's stamped auth field. Only a route the
// SERVER marked "unauthenticated" is a finding — an "unknown" (custom middleware
// the classifier declined to guess, ADR 0002) is deliberately NOT flagged, and
// neither is an authenticated route.
function isUnauthenticated(route) {
  return route.auth === "unauthenticated";
}

// authVerdict summarizes auth coverage from the routes the contract carries: how
// many are unauthenticated, out of how many total, and how many of the
// unauthenticated ones are writes. It returns those counts plus a ready-to-render
// sentence in the tool's calm, specific voice. Pure (routes in, summary out) so
// the sentence is unit-testable without a DOM.
export function authVerdict(routes) {
  const list = routes || [];
  const unauth = list.filter(isUnauthenticated);
  const writes = unauth.filter((r) => isWriteMethod(r.method)).length;

  const summary = {
    total: list.length,
    unauthenticated: unauth.length,
    writes,
  };
  summary.sentence = verdictSentence(summary);
  return summary;
}

// verdictSentence renders the verdict counts into one sentence. An all-clear
// project (nothing unauthenticated) gets a plainly positive line; otherwise the
// sentence names the unauthenticated count, the total, and how many write —
// facts, stated once, no exclamation.
function verdictSentence({ total, unauthenticated, writes }) {
  if (unauthenticated === 0) {
    return "Every route is behind auth middleware.";
  }
  const writeClause = writes > 0 ? `, ${writes} of them write` : "";
  return `${unauthenticated} of ${total} routes have no auth middleware${writeClause}.`;
}

// groupByStack buckets the unauthenticated routes by their middleware stack, so
// the view can show "these public routes share this (non-auth) stack". The stack
// key is the joined middleware names; a route with none lands under
// STACK_NONE_LABEL. Groups and the routes within them preserve source order, so
// the render is deterministic. Authenticated and unknown routes are excluded —
// only the findings are grouped.
export function groupByStack(routes) {
  const order = [];
  const byStack = new Map();
  for (const route of routes || []) {
    if (!isUnauthenticated(route)) continue;
    const stack = (route.middleware && route.middleware.length)
      ? route.middleware.join(", ")
      : STACK_NONE_LABEL;
    if (!byStack.has(stack)) {
      byStack.set(stack, []);
      order.push(stack);
    }
    byStack.get(stack).push(route);
  }
  return order.map((stack) => ({ stack, routes: byStack.get(stack) }));
}

// authRowHtml is the pure per-route template. Severity reads as a leading status
// dot whose color is backed up by spelled-out text: a write route is the red
// danger dot AND says "write", a read is the amber warn dot AND says "read". The
// dot is aria-hidden because the row's own text carries the meaning (the
// non-color channel); the URI links to the Routes view filtered to that path.
// No side-stripe (ADR 0011 / issue #38). Exported so the markup contract is
// unit-testable without a DOM, mirroring findingRowHtml.
export function authRowHtml(route, currentParams) {
  const write = isWriteMethod(route.method);
  const dot = write ? "danger" : "warn";
  const kindWord = write ? "write" : "read";
  const href = hrefFor("route", route.uri, currentParams);
  return (
    '<div class="finding' + (write ? " dead" : "") + '">' +
    '<div class="h">' +
    '<span class="status-dot ' + dot + '" aria-hidden="true"></span>' +
    "Unauthenticated " + kindWord + ": " +
    '<a href="' + escapeHtml(href) + '">' +
    escapeHtml((route.method || "") + " " + (route.uri || "")) +
    "</a></div></div>"
  );
}

// renderAuth paints the Auth panel from the analysis model (issue #50): it sets
// the nav badge to the unauthenticated count, writes the verdict sentence, and
// lists each middleware-stack group with its routes. It reads model.routes[].auth
// — the server-stamped field — so nothing is re-classified here (ADR 0008).
export function renderAuth(routes, currentParams) {
  const list = routes || [];
  const verdict = authVerdict(list);

  const badge = $("#badge-auth");
  if (badge) badge.textContent = verdict.unauthenticated;

  const body = $("#auth-body");
  if (!body) return;

  const groups = groupByStack(list);
  const groupsHtml = groups
    .map(
      (g) =>
        '<section class="auth-group">' +
        '<h3 class="auth-stack">' + escapeHtml(g.stack) + "</h3>" +
        g.routes.map((r) => authRowHtml(r, currentParams)).join("") +
        "</section>"
    )
    .join("");

  body.innerHTML =
    '<p class="auth-verdict">' + escapeHtml(verdict.sentence) + "</p>" + groupsHtml;
}
