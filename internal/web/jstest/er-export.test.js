// Unit tests for the ER diagram export helpers (issue #30).
//
// er-export.js turns the mounted, themed ER <svg> into a standalone artifact
// that renders correctly OUTSIDE the app — in a docs page, a code review, an
// image viewer — where none of the app's stylesheets or CSS custom properties
// are present. The load-bearing fact is therefore self-containment: the
// exported markup must carry its own resolved styling, because a var(--accent)
// or an external .er-box rule means nothing once the SVG leaves the page.
//
// The pure pieces (standalone markup, dimension/scale math) are unit-tested
// here without a browser; the actual PNG rasterization and file download are
// DOM/canvas-bound and are wired-and-verified in er.js, not unit-tested (Node's
// built-in runner has no canvas). pngBlob's *pre-canvas* contract — the scaled
// pixel dimensions it will draw at — is pure and IS tested here.
//
//   node --test 'internal/web/jstest/**/*.test.js'

import { test } from "node:test";
import assert from "node:assert/strict";

import { standaloneSvg, scaledDimensions } from "../assets/js/views/er-export.js";

// A minimal stand-in for a mounted, themed <svg> element. The real export path
// reads these off a live SVGElement; the pure helper only needs outerHTML plus
// the resolved style declarations it should inline, so the stub exposes exactly
// those. viewBox carries the intrinsic diagram size.
function fakeSvg({ outer, styles = "" } = {}) {
  return {
    outerHTML:
      outer ||
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 220 78" class="er-svg">' +
        '<g class="er-nodes"></g></svg>',
    resolvedStyles: styles,
  };
}

test("standaloneSvg returns markup with an SVG namespace so it opens on its own", () => {
  const out = standaloneSvg(fakeSvg());
  assert.match(out, /^<svg\b/);
  assert.match(out, /xmlns="http:\/\/www\.w3\.org\/2000\/svg"/);
});

test("scaledDimensions rasterizes the viewBox size, scaled up for a crisp PNG", () => {
  // A PNG has no vectors — it's baked at a fixed pixel size. Exporting at the
  // intrinsic viewBox size (220×78) would look soft; a scale factor gives a
  // retina-crisp raster. The math is pure, so it's tested without a canvas.
  const dims = scaledDimensions("0 0 220 78", 2);
  assert.deepEqual(dims, { width: 440, height: 156 });
});

test("scaledDimensions falls back to a sane size when the viewBox is missing", () => {
  // A malformed or absent viewBox must not yield a 0×0 (canvas throws) or NaN
  // canvas — export degrades to a non-empty default rather than crashing.
  const dims = scaledDimensions("", 2);
  assert.ok(dims.width > 0 && dims.height > 0, "empty viewBox produced a zero-area canvas");
});

test("standaloneSvg inlines the resolved styling so it renders without the app CSS", () => {
  // The app themes the diagram via external stylesheets + CSS custom
  // properties. Once exported, none of that is present — so the concrete,
  // token-resolved rules must travel INSIDE the SVG as a <style> block.
  const out = standaloneSvg(
    fakeSvg({ styles: ".er-box{fill:#262626;stroke:#525252}" })
  );
  assert.match(out, /<style>[\s\S]*\.er-box\{fill:#262626;stroke:#525252\}[\s\S]*<\/style>/);
  // No unresolved custom properties leak into the artifact — a var() means
  // nothing outside the app and would render unstyled.
  assert.ok(!/var\(--/.test(out), "export leaked an unresolved CSS variable");
});
