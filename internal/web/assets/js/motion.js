// Shared motion plumbing for the settle gesture (ADR 0009, promoted app-wide
// by ADR 0010). Every JS-driven settle consults prefersReducedMotion() before
// choreographing anything — base.css's reduced-motion kill switch is the CSS
// backstop, this is the JS half of the double guard.

// prefersReducedMotion reflects the OS/browser "reduce motion" setting. Guarded
// so a non-browser context (or a browser without matchMedia) simply reports
// false rather than throwing.
export function prefersReducedMotion() {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

// SETTLE_CLEANUP_MS is the fallback timer that tears down a settle's staged
// state if transitionend never fires. Comfortably longer than --motion-settle
// (620ms) so it only ever acts as a safety net.
export const SETTLE_CLEANUP_MS = 1200;
