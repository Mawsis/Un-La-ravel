// er-export.js turns the mounted, themed ER <svg> into a standalone artifact
// (issue #30) that renders correctly OUTSIDE the app — a docs page, a code
// review, an image viewer — where the app's stylesheets and CSS custom
// properties don't exist. The design constraint is self-containment: the
// exported markup carries its own resolved styling, because a var(--resolved) or
// an external .er-box rule resolves to nothing once the SVG leaves the page.
//
// standaloneSvg is a pure string transform (unit-tested, no DOM); the PNG path
// and file download are DOM/canvas-bound and wired in er.js.

// standaloneSvg produces a self-contained SVG string from the mounted diagram.
// `svgEl` is the live <svg> element (or a stub exposing the same surface):
// `.outerHTML` is the drawn markup and `.resolvedStyles` is a CSS block with the
// diagram's token-derived styling already resolved to concrete values, inlined
// so the export needs no external stylesheet.
// Fallback raster size when the SVG carries no usable viewBox — a non-empty
// canvas the browser can actually draw to, rather than a 0×0 (which throws) or
// a NaN dimension.
const FALLBACK_SIZE = 800;

// scaledDimensions computes the PNG's pixel dimensions from the SVG viewBox
// ("minX minY width height") multiplied by `scale`. A PNG bakes to a fixed
// pixel grid, so exporting at the intrinsic vector size looks soft; scaling up
// gives a crisp raster. A missing/malformed viewBox degrades to a square
// fallback rather than a zero-area or NaN canvas.
export function scaledDimensions(viewBox, scale) {
  const parts = String(viewBox || "").trim().split(/\s+/).map(Number);
  const w = parts.length === 4 && parts[2] > 0 ? parts[2] : FALLBACK_SIZE;
  const h = parts.length === 4 && parts[3] > 0 ? parts[3] : FALLBACK_SIZE;
  return { width: Math.round(w * scale), height: Math.round(h * scale) };
}

export function standaloneSvg(svgEl) {
  const markup = svgEl.outerHTML;
  const styles = svgEl.resolvedStyles || "";
  if (!styles) return markup;

  // Insert the resolved styling as the SVG's first child, immediately after the
  // opening tag, so it applies to everything drawn below it. An inline <style>
  // travels with the file — external rules and var(--…) references would not.
  return markup.replace(/(<svg\b[^>]*>)/, `$1<style>${styles}</style>`);
}

// ---------------------------------------------------------------------------
// DOM-bound export path (issue #30). These functions read computed styles off
// the live diagram, serialize it, and trigger a browser download. They're
// browser-only (getComputedStyle, canvas, Blob, <a download>), so they're
// wired-and-verified against the fixture app rather than unit-tested — Node's
// built-in test runner has no DOM or canvas. Every pure decision they make (the
// standalone markup, the raster dimensions) delegates to the tested helpers
// above, keeping the untested surface as thin as possible.
// ---------------------------------------------------------------------------

// The ER classes whose computed styling must travel with an export. getComputed
// Style resolves each to concrete values (var(--…) already substituted), which
// standaloneSvg inlines as a <style> block so the artifact renders standalone.
const EXPORT_STYLE_SELECTORS = [
  ".er-box",
  ".er-header",
  ".er-title",
  ".er-col-name",
  ".er-col-type",
  ".er-badge",
  ".er-badge-pk",
  ".er-badge-fk",
  ".er-badge-uk",
  ".er-edge",
  ".er-edge-eloquent",
  ".er-edge-schema",
  ".er-marker",
];

// The style properties worth capturing per selector — the visual surface of the
// diagram (fill/stroke/type), not layout. Kept explicit so the exported <style>
// stays small and legible rather than dumping every computed property.
const EXPORT_STYLE_PROPS = [
  "fill",
  "stroke",
  "stroke-width",
  "stroke-dasharray",
  "font-family",
  "font-size",
  "font-weight",
];

// serialize builds the self-contained SVG string for `svgEl`, resolving the
// diagram's token-driven styling to concrete values via getComputedStyle so the
// exported markup carries its own look (issue #30: "exports reflect the current
// diagram styling"). A DOM document is required to resolve computed styles, so
// this is the DOM-bound entry the pure standaloneSvg sits under.
function serialize(svgEl) {
  const styled = { outerHTML: svgEl.outerHTML, resolvedStyles: collectStyles(svgEl) };
  return standaloneSvg(styled);
}

// collectStyles resolves each export selector to a concrete CSS rule by reading
// getComputedStyle off a representative element in the live SVG. A selector with
// no matching element is skipped (nothing to style).
function collectStyles(svgEl) {
  const doc = svgEl.ownerDocument;
  const view = (doc && doc.defaultView) || window;
  return EXPORT_STYLE_SELECTORS.map((sel) => {
    const el = svgEl.querySelector(sel);
    if (!el) return "";
    const cs = view.getComputedStyle(el);
    const decls = EXPORT_STYLE_PROPS.map((p) => {
      const v = cs.getPropertyValue(p);
      return v ? `${p}:${v.trim()}` : "";
    }).filter(Boolean);
    return decls.length ? `${sel}{${decls.join(";")}}` : "";
  })
    .filter(Boolean)
    .join("");
}

// downloadSvg exports the diagram as a standalone .svg file (vector, infinitely
// crisp, editable). `filename` names the download; it defaults to "er-diagram".
export function downloadSvg(svgEl, filename = "er-diagram") {
  if (!svgEl) return;
  const markup = serialize(svgEl);
  const blob = new Blob([markup], { type: "image/svg+xml;charset=utf-8" });
  triggerDownload(URL.createObjectURL(blob), `${filename}.svg`, true);
}

// downloadPng rasterizes the diagram to a PNG at `scale`× its intrinsic size
// (default 2× for a crisp result) and downloads it. The SVG is drawn onto a
// canvas via an Image, then read back as a PNG blob. Async because Image load
// is async; failures surface on the console rather than throwing into a click
// handler.
export function downloadPng(svgEl, { filename = "er-diagram", scale = 2 } = {}) {
  if (!svgEl) return;
  const markup = serialize(svgEl);
  const { width, height } = scaledDimensions(svgEl.getAttribute("viewBox"), scale);

  const svgUrl = URL.createObjectURL(new Blob([markup], { type: "image/svg+xml;charset=utf-8" }));
  const img = new Image();
  img.onload = () => {
    try {
      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext("2d");
      ctx.drawImage(img, 0, 0, width, height);
      canvas.toBlob((blob) => {
        URL.revokeObjectURL(svgUrl);
        if (!blob) return;
        triggerDownload(URL.createObjectURL(blob), `${filename}.png`, true);
      }, "image/png");
    } catch (e) {
      URL.revokeObjectURL(svgUrl);
      // eslint-disable-next-line no-console
      console.error("ER PNG export failed:", e);
    }
  };
  img.onerror = () => {
    URL.revokeObjectURL(svgUrl);
    // eslint-disable-next-line no-console
    console.error("ER PNG export failed: could not load the SVG for rasterization");
  };
  img.src = svgUrl;
}

// triggerDownload clicks a synthetic <a download> to save `url` as `name`, then
// revokes the object URL if we own it (revokeOwned) so it doesn't leak. The
// revoke sits in a `finally` so a throw from appendChild/click (e.g. the SPA
// tore the view down mid-export, or a locked-down environment blocks the click)
// can't leak the object URL — the whole reason downloadPng passes revokeOwned
// from inside an async toBlob callback that has no surrounding catch.
function triggerDownload(url, name, revokeOwned) {
  try {
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    document.body.appendChild(a);
    a.click();
    a.remove();
  } finally {
    if (revokeOwned) URL.revokeObjectURL(url);
  }
}
