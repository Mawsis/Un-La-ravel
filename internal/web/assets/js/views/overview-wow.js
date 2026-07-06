// Overview wow moment (issue #39 variant A, committed via issue #44): the
// Laravel mark holds in brand red, its strokes loosen into unresolved-red
// points, the points fly to the project's constellation (overview-
// constellation.js), threads draw red and resolve green as their endpoints
// land, the stat cards tally in sync with the settled fraction, and the
// verdict closes the moment. The one brand-register surface: loud here,
// nowhere else.
//
// Degradation is part of the contract, not an afterthought:
//   - prefers-reduced-motion: no choreography at all — the static resolved
//     constellation is drawn, the cards keep their final numbers, the
//     verdict is simply present;
//   - Overview not visible when the analysis lands (deep link to another
//     view): the moment is held and plays on the first visit to Overview
//     for that analysis — once per analysis, like the sidebar thread-mark.
//
// All colors are read from the token layer at play time (getComputedStyle),
// so the canvas can never drift from tokens.css.

import { $ } from "../dom.js";
import { prefersReducedMotion } from "../motion.js";
import { buildConstellation, mulberry32, KIND_KEYS } from "./overview-constellation.js";
import { assignEntities, hitTest, entityListHtml } from "./overview-hit.js";
import * as router from "../router.js";

// Timeline (ms). The whole take is ~4s: cinematic for first contact, short
// enough that a replay per analysis never feels like a loading screen.
const T_HOLD = 500; // the mark, whole, in brand red
const T_UNRAVEL = 1100; // strokes loosen into loose red points
const T_SETTLE_SPAN = 1700; // stagger window for departures
const T_NODE = 850; // one point's flight time
const T_END = T_HOLD + T_UNRAVEL + T_SETTLE_SPAN + T_NODE;

// The mark's own geometry (the same paths as the hero/sidebar SVG), sampled
// into points via a hidden SVG — hand-rolling bezier math would just restate
// what the browser already knows.
const MARK_PATHS = [
  "M57 214 L121 150 L309 338 L245 402 Z",
  "M284 198 L340 255 L222 374 L166 317 Z",
  "M165 334 L223 392 L165 450 L106 392 Z",
  "M245 402 C 285 442, 325 420, 365 418 C 405 416, 425 424, 470 424 L 588 424",
];
const MARK_W = 640;
const MARK_H = 470;

let pending = null; // { counts, entities } awaiting a visible Overview, or null
let raf = 0;

// entityNames pulls each kind's display names from the model in source order,
// keyed by KIND_KEYS so overview-hit.js can bind them to the drawn dots. Routes
// have no single name field, so a route reads as "METHOD /uri" — the same shape
// the search index and route rows use — so hovering a route dot names it
// legibly. counts derive from these lists, keeping the tally and the hit-test
// binding reading from one source.
function entityNames(model) {
  const m = model || {};
  return {
    tables: (m.schemas || []).map((t) => t.name || ""),
    models: (m.models || []).map((x) => x.name || ""),
    controllers: (m.controllers || []).map((c) => c.name || ""),
    routes: (m.routes || []).map((r) => ((r.method || "") + " " + (r.uri || "")).trim()),
    requests: (m.form_requests || []).map((f) => f.name || ""),
  };
}

// prepareOverviewWow is called once per analysis (main.js's once-per-analysis
// block). If Overview is visible it plays now; otherwise the take is held for
// the first visit.
export function prepareOverviewWow(model) {
  const entities = entityNames(model);
  pending = {
    entities,
    counts: {
      tables: entities.tables.length,
      models: entities.models.length,
      controllers: entities.controllers.length,
      routes: entities.routes.length,
      requests: entities.requests.length,
    },
  };
  maybePlayOverviewWow();
}

// maybePlayOverviewWow runs the held take if the canvas is actually laid out
// (a hidden panel measures 0×0). Called on prepare and on every activation of
// the Overview view.
export function maybePlayOverviewWow() {
  if (!pending) return;
  const canvas = $("#overview-canvas");
  if (!canvas) return;
  const rect = canvas.getBoundingClientRect();
  if (rect.width === 0 || rect.height === 0) return; // not visible yet — hold

  const { counts, entities } = pending;
  pending = null;
  cancelAnimationFrame(raf);

  const dpr = window.devicePixelRatio || 1;
  const w = rect.width;
  const h = rect.height;
  canvas.width = w * dpr;
  canvas.height = h * dpr;
  const ctx = canvas.getContext("2d");
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

  const colors = threadColors(canvas);
  const scene = buildScene(counts, entities, w, h);
  const verdict = $("#verdict");

  // Publish the equivalent keyboard/AT path up front (issue #52). The dot→entity
  // binding is already on the scene (buildScene), so hit-testing holds whether
  // the take animates or is skipped under reduced motion.
  renderEntityList(entities);

  if (prefersReducedMotion()) {
    drawFrame(ctx, scene, T_END + 1000, w, h, colors);
    bindConstellationInteraction(canvas, scene, w, h); // static end-state is fully interactive
    return; // cards already hold final values; verdict is simply present
  }

  if (verdict) verdict.classList.add("wow-pending");
  const cards = tallyTargets(counts);
  setTally(cards, 0);

  const t0 = performance.now();
  const tick = (now) => {
    const t = now - t0;
    const settleClock = drawFrame(ctx, scene, t, w, h, colors);
    setTally(cards, settleClock / (T_SETTLE_SPAN + T_NODE));
    if (verdict && t >= T_END - 350) {
      verdict.classList.add("wow-shown");
      verdict.classList.remove("wow-pending");
    }
    // The wow moment stays a look-don't-touch take until it lands; after the
    // settle, the same dots become navigable (issue #52 — the choreography is
    // unchanged, interaction begins once it completes).
    if (t >= T_END && !scene.interactive) {
      scene.interactive = true;
      bindConstellationInteraction(canvas, scene, w, h);
    }
    if (t < T_END + 120) raf = requestAnimationFrame(tick);
  };
  raf = requestAnimationFrame(tick);
}

// ---- scene ------------------------------------------------------------------

// buildScene marries the constellation end-state with the mark's sampled
// start points: node i starts life as a point on the mark's strokes, drifts
// loose during the unravel, then flies to its constellation seat. It also binds
// each dot to its real entity (assignEntities, issue #52) so the SAME node
// objects the strokes and edges reference carry the identity hit-testing reads —
// one array, no aliasing between the drawn scene and the interactive scene.
function buildScene(counts, entities, w, h) {
  const built = buildConstellation(counts, w, h);
  const nodes = assignEntities(built.nodes, entities);
  const edges = built.edges;
  const markPts = sampleMarkPoints(Math.max(nodes.length, 1));
  const s = Math.min(w / MARK_W, h / MARK_H) * 0.72;
  const ox = w / 2 - (MARK_W / 2) * s;
  const oy = h / 2 - (MARK_H / 2) * s;
  const rand = mulberry32(7);
  nodes.forEach((n, i) => {
    const m = markPts[i % markPts.length];
    n.mx = ox + m.x * s;
    n.my = oy + m.y * s;
    n.ux = n.mx + (rand() - 0.5) * 70;
    n.uy = n.my + (rand() - 0.5) * 70;
    n.delay = rand() * T_SETTLE_SPAN;
    n.path = m.path;
    n.along = m.along;
  });
  const strokes = MARK_PATHS.map((_, pi) =>
    nodes.filter((n) => n.path === pi).sort((a, b) => a.along - b.along)
  );
  return { nodes, edges, strokes };
}

// sampleMarkPoints distributes `total` points along the mark's four strokes,
// proportional to each stroke's length, using a hidden in-DOM SVG for
// getPointAtLength (geometry APIs need a live element).
function sampleMarkPoints(total) {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("viewBox", `0 0 ${MARK_W} ${MARK_H}`);
  svg.setAttribute("aria-hidden", "true");
  svg.style.position = "absolute";
  svg.style.width = "0";
  svg.style.height = "0";
  const paths = MARK_PATHS.map((d) => {
    const p = document.createElementNS("http://www.w3.org/2000/svg", "path");
    p.setAttribute("d", d);
    svg.appendChild(p);
    return p;
  });
  document.body.appendChild(svg);
  const lens = paths.map((p) => p.getTotalLength());
  const totalLen = lens.reduce((a, b) => a + b, 0) || 1;
  const pts = [];
  paths.forEach((p, pi) => {
    const n = Math.max(4, Math.round((lens[pi] / totalLen) * total));
    for (let i = 0; i < n; i++) {
      const pt = p.getPointAtLength((i / (n - 1)) * lens[pi]);
      pts.push({ x: pt.x, y: pt.y, path: pi, along: i / (n - 1) });
    }
  });
  svg.remove();
  return pts;
}

// ---- drawing ------------------------------------------------------------------

function drawFrame(ctx, scene, t, w, h, colors) {
  ctx.clearRect(0, 0, w, h);
  const unravelT = easeOut((t - T_HOLD) / T_UNRAVEL);
  const settleClock = t - T_HOLD - T_UNRAVEL;

  if (unravelT < 1) {
    ctx.lineWidth = 9 * (1 - unravelT * 0.7);
    ctx.lineCap = ctx.lineJoin = "round";
    ctx.strokeStyle = rgba(lerpC(colors.brand, colors.unresolved, unravelT), 1 - unravelT);
    for (const pts of scene.strokes) {
      if (pts.length < 2) continue;
      ctx.beginPath();
      pts.forEach((n, i) => {
        const x = lerp(n.mx, n.ux, unravelT);
        const y = lerp(n.my, n.uy, unravelT);
        i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
      });
      ctx.stroke();
    }
  }

  for (const [ai, bi] of scene.edges) {
    const a = scene.nodes[ai];
    const b = scene.nodes[bi];
    const p = Math.min(nodeProgress(a, settleClock), nodeProgress(b, settleClock));
    if (p <= 0) continue;
    ctx.lineWidth = 1;
    ctx.strokeStyle = rgba(lerpC(colors.unresolved, colors.resolved, p), 0.1 + 0.16 * p);
    ctx.beginPath();
    ctx.moveTo(nodeX(a, settleClock), nodeY(a, settleClock));
    ctx.lineTo(nodeX(b, settleClock), nodeY(b, settleClock));
    ctx.stroke();
  }

  for (const n of scene.nodes) {
    const p = nodeProgress(n, settleClock);
    const vis = Math.max(unravelT * 0.9, p > 0 ? 1 : 0);
    if (vis <= 0) continue;
    ctx.fillStyle = rgba(lerpC(colors.unresolved, colors.resolved, p), 0.35 + 0.65 * Math.max(p, unravelT * 0.5));
    ctx.beginPath();
    ctx.arc(nodeX(n, settleClock), nodeY(n, settleClock), n.size * (0.8 + 0.4 * p), 0, Math.PI * 2);
    ctx.fill();
  }

  return settleClock;
}

function nodeProgress(n, clock) {
  return easeOut((clock - n.delay) / T_NODE);
}
function nodeX(n, clock) {
  return lerp(n.ux, n.tx, nodeProgress(n, clock));
}
function nodeY(n, clock) {
  return lerp(n.uy, n.ty, nodeProgress(n, clock));
}

// ---- tally ----------------------------------------------------------------

// tallyTargets pairs each kind's true count with its card's number element.
// overview.js renders the cards in KIND_KEYS order (Tables, Models,
// Controllers, Routes, Form requests), so index pairing holds.
function tallyTargets(counts) {
  const els = Array.from(document.querySelectorAll("#cards .card"));
  return KIND_KEYS.map((k, i) => ({
    count: counts[k],
    card: els[i] || null,
  }));
}

function setTally(targets, fraction) {
  const f = easeOut(fraction);
  for (const t of targets) {
    if (!t.card) continue;
    const n = t.card.querySelector(".n");
    if (n) n.textContent = Math.round(t.count * f);
    t.card.classList.toggle("tallying", f > 0 && f < 1);
  }
}

// ---- interaction (issue #52) ------------------------------------------------

// renderEntityList publishes the visually-hidden, focusable, linked entity list
// beside the canvas — the equivalent keyboard/AT path so the map is never
// mouse-only. The markup is built by the pure entityListHtml (overview-hit.js),
// which the jstests cover; this is only the DOM write.
function renderEntityList(entities) {
  const host = $("#constellation-entities");
  if (host) host.innerHTML = entityListHtml(entities);
}

// bindConstellationInteraction gives the settled dots their two behaviors: hover
// names the entity under the pointer, a plain left-click on a MODEL dot opens
// its detail page (#/models/{name}, issue #51). Only models navigate — other
// kinds hover-name but have no detail page, matching chipTarget and the rest of
// the app. Re-binding replaces prior handlers (a re-analysis rebuilds the scene),
// so listeners never stack up across analyses.
let detachInteraction = null;
function bindConstellationInteraction(canvas, scene, w, h) {
  if (detachInteraction) detachInteraction();
  const tip = $("#constellation-tip");

  // The canvas backing store is DPR-scaled and CSS-stretched to w×h; map a
  // client pointer position into the same w×h space the scene's tx/ty live in.
  const toScene = (e) => {
    const r = canvas.getBoundingClientRect();
    return {
      x: ((e.clientX - r.left) / r.width) * w,
      y: ((e.clientY - r.top) / r.height) * h,
    };
  };

  const onMove = (e) => {
    const { x, y } = toScene(e);
    const hit = hitTest(scene.nodes, x, y);
    if (hit && hit.entityName) {
      showTip(tip, hit.entityName, x, y, w);
      canvas.style.cursor = hit.entityKind === "models" ? "pointer" : "default";
    } else {
      hideTip(tip);
      canvas.style.cursor = "default";
    }
  };

  const onLeave = () => {
    hideTip(tip);
    canvas.style.cursor = "default";
  };

  const onClick = (e) => {
    if (!isPlainLeftClick(e)) return; // let cmd/ctrl/shift/middle keep native behavior
    const { x, y } = toScene(e);
    const hit = hitTest(scene.nodes, x, y);
    // Only a model dot navigates (it has a detail page); other kinds hover-name
    // but click is inert, the confirmed contract mirroring chipTarget.
    if (hit && hit.entityName && hit.entityKind === "models") {
      e.preventDefault();
      navigateToModel(hit.entityName);
    }
  };

  canvas.addEventListener("mousemove", onMove);
  canvas.addEventListener("mouseleave", onLeave);
  canvas.addEventListener("click", onClick);
  detachInteraction = () => {
    canvas.removeEventListener("mousemove", onMove);
    canvas.removeEventListener("mouseleave", onLeave);
    canvas.removeEventListener("click", onClick);
    detachInteraction = null;
  };
}

// isPlainLeftClick mirrors main.js: only a bare left-click is intercepted, so
// middle/cmd/ctrl/shift-click keep the browser's open-in-new-tab gestures — the
// AT list's real <a> chips are what make those gestures land somewhere.
function isPlainLeftClick(e) {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
}

// navigateToModel routes to a model's detail page the same way the delegated
// entity-chip handler does (main.js): carry the current ?path= forward, drop
// the view-specific params, and set the detail segment — one shareable URL
// change, consistent with every other in-app jump.
function navigateToModel(name) {
  const { params } = router.getCurrent();
  const next = new URLSearchParams();
  if (params.get("path")) next.set("path", params.get("path"));
  router.navigate("models", next, { detail: name });
}

function showTip(tip, text, x, y, w) {
  if (!tip) return;
  tip.textContent = text;
  tip.hidden = false;
  // Anchor above the dot; flip horizontal alignment near the right edge so the
  // label never spills off-canvas.
  tip.style.left = x + "px";
  tip.style.top = y + "px";
  tip.classList.toggle("flip", x > w * 0.75);
}

function hideTip(tip) {
  if (tip) tip.hidden = true;
}

// ---- color/math helpers -------------------------------------------------------

// threadColors resolves --brand/--unresolved/--resolved to RGB by letting the
// canvas itself parse whatever tokens.css authored (oklch included): fill a
// 1×1 canvas and read the pixel back. Single source of truth stays tokens.css.
function threadColors(canvas) {
  const styles = getComputedStyle(canvas);
  const probe = document.createElement("canvas");
  probe.width = probe.height = 1;
  const pctx = probe.getContext("2d", { willReadFrequently: true });
  const resolve = (name) => {
    pctx.fillStyle = styles.getPropertyValue(name).trim() || "#808080";
    pctx.fillRect(0, 0, 1, 1);
    const d = pctx.getImageData(0, 0, 1, 1).data;
    return [d[0], d[1], d[2]];
  };
  return {
    brand: resolve("--brand"),
    unresolved: resolve("--unresolved"),
    resolved: resolve("--resolved"),
  };
}

function lerp(a, b, t) {
  return a + (b - a) * t;
}
function lerpC(a, b, t) {
  return [lerp(a[0], b[0], t), lerp(a[1], b[1], t), lerp(a[2], b[2], t)];
}
function rgba(c, a) {
  return `rgba(${c[0] | 0},${c[1] | 0},${c[2] | 0},${a})`;
}
// Approximates --motion-ease (0.2, 0, 0, 1): fast start, long landing.
function easeOut(t) {
  return 1 - Math.pow(1 - Math.min(Math.max(t, 0), 1), 3);
}
