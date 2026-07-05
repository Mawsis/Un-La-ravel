// Recent-projects list on the entry screen (design.md "URL & state": "The
// entry screen gains a recent-projects list (from store) with remove
// buttons"). Shown before any analysis, alongside the intro hint. Reads
// store.js's localStorage-backed list — purely a convenience shortcut, never
// load-bearing: an empty or unavailable store just means an empty list here.

import { $, escapeHtml } from "../dom.js";
import { getRecents, removeRecent } from "../store.js";

// renderRecents draws the list into #recents. onSelect(path) fires when the
// user clicks a recent project's path (the caller re-analyzes it);
// re-rendering after a removal is handled internally so callers don't need
// to know the list changed.
export function renderRecents(onSelect) {
  const container = $("#recents");
  if (!container) return;

  const recents = getRecents();
  if (recents.length === 0) {
    container.innerHTML = "";
    return;
  }

  container.innerHTML =
    '<div class="subhead">Recent projects</div><ul class="recents-list">' +
    recents
      .map(
        (r) =>
          '<li><button type="button" class="recent-path" data-path="' + escapeHtml(r.path) + '">' +
          escapeHtml(r.projectName || r.path) +
          '</button><button type="button" class="recent-remove" data-path="' + escapeHtml(r.path) + '" aria-label="Remove ' + escapeHtml(r.path) + ' from recent projects">&times;</button></li>'
      )
      .join("") +
    "</ul>";

  container.querySelectorAll(".recent-path").forEach((btn) => {
    btn.onclick = () => onSelect(btn.dataset.path);
  });
  container.querySelectorAll(".recent-remove").forEach((btn) => {
    btn.onclick = () => {
      removeRecent(btn.dataset.path);
      renderRecents(onSelect);
    };
  });
}
