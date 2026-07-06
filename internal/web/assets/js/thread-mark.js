// Sidebar thread-mark (issue #39): the settle's app-wide signature, kept
// scarce — a small thread in the brand block that re-settles exactly once per
// new analysis (called from main.js's once-per-analysis render block, the same
// cadence as the health chip). Steady state is the resolved green thread; the
// replay flashes the tangled red state and lets a token-timed transition
// (components.css, --motion-settle) crossfade it back to resolved. Same
// FLIP-style arm-then-flip idiom as the ER settle (ADR 0009): snap the start
// state without transitions, then arm them on the next frame so the browser
// has a start frame to interpolate from.

import { $ } from "./dom.js";
import { prefersReducedMotion, SETTLE_CLEANUP_MS } from "./motion.js";

export function resettleThreadMark() {
  const mark = $("#thread-mark");
  if (!mark) return;
  // Reduced motion: no replay — the mark already sits at the static resolved
  // end-state, which is the degradation DESIGN.md §6 prescribes.
  if (prefersReducedMotion()) return;

  // A re-analysis can land mid-flight: drop any armed transition first so the
  // snap back to tangled is instant, not an animated regression.
  mark.classList.remove("tm-resolving");
  mark.classList.add("tm-tangled-start");
  void mark.getBoundingClientRect(); // commit the tangled start frame

  requestAnimationFrame(() => {
    mark.classList.add("tm-resolving"); // arm the settle transition…
    mark.classList.remove("tm-tangled-start"); // …and flip to resolved

    let done = false;
    const once = () => {
      if (done) return;
      done = true;
      mark.classList.remove("tm-resolving");
    };
    mark.addEventListener("transitionend", once, { once: true });
    setTimeout(once, SETTLE_CLEANUP_MS);
  });
}
