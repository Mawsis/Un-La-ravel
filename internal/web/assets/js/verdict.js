// The Overview health-verdict renderer (issue #27): the core UX fix that makes
// problems out-shout inventory. healthVerdict reads the honest, itemized verdict
// the SERVER computed into the model — never a fabricated score or letter grade,
// and, as of contract 1.6.0, never recomputed in the browser.
//
// The verdict now lives in one place: internal/findings computes it into the
// Project Model, engine.Analyze serializes it as model.findings, and this reader
// simply surfaces that array. So the CLI's `doctor` command, the unlaravel.json
// contract, and this dashboard all report the SAME verdict by construction — the
// browser can no longer drift from the server the way a re-implementation could.
//
// Kept pure (no DOM) so it is unit-testable on its own and can't drift from the
// markup that consumes it, mirroring the chipTarget/entityChip split in chip.js.

// healthVerdict maps an analyzed model to its health verdict by reading the
// server-computed findings array. Each finding is one problem category with a
// nonzero count, already carrying its machine-readable `kind`, `count`, a
// pluralized human `label`, and the `view` to link to (the findings view) — the
// exact shape the Overview markup consumes. A project is clean when the findings
// array is empty (or absent). The order is the server's fixed
// understand-then-judge order (dead routes → disagreements → unguarded models);
// this reader preserves it.
export function healthVerdict(model) {
  const problems = findingsOf(model);
  return { clean: problems.length === 0, problems };
}

// healthChip summarizes the same server-computed findings into the persistent
// sidebar chip's state (issue #23): clean, or one total issue count. Like
// healthVerdict, it reads model.findings — never recomputing from raw counts.
export function healthChip(model) {
  const problems = findingsOf(model);
  const count = problems.reduce((sum, p) => sum + (p.count || 0), 0);
  if (count === 0) {
    return { clean: true, count: 0, label: "clean" };
  }
  return { clean: false, count, label: count + (count === 1 ? " issue" : " issues") };
}

// findingsOf is the one place both readers pull the server-computed findings
// array from the model, absent-key-safe.
function findingsOf(model) {
  return (model && model.findings) || [];
}
