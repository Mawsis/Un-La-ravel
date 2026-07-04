// Sidebar health rendering (issue #23): the persistent chip near the brand and
// the Findings count badge, both driven by the SAME server-computed findings
// (healthChip in verdict.js), so the chip and the badge cannot disagree. Called
// once per analysis from main.js — this is how both "update live when a new
// project is analyzed."

import { $ } from "./dom.js";
import { healthChip } from "./verdict.js";

export function renderSidebarHealth(model) {
  const chip = healthChip(model);

  const el = $("#health-chip");
  el.hidden = false;
  el.textContent = chip.clean ? "✓ clean" : "⚠ " + chip.label;
  el.classList.toggle("is-clean", chip.clean);
  el.classList.toggle("is-issues", !chip.clean);

  const badge = $("#badge-findings");
  badge.textContent = chip.count;
  badge.classList.toggle("danger", chip.count > 0);
}
