// The Overview health-verdict renderer (issue #27): the core UX fix that makes
// problems out-shout inventory. healthVerdict computes an honest, itemized
// verdict from the analyzed model — never a fabricated score or letter grade.
//
// Kept pure (no DOM) so it is unit-testable on its own and can't drift from the
// markup that consumes it, mirroring the chipTarget/entityChip split in chip.js.

// healthVerdict maps an analyzed model to its health verdict: whether the
// project is clean, and if not, an itemized breakdown by category. Each problem
// category counts a distinct static finding:
//
//   - dead routes  — routes whose controller/action edge dangles (model.dead_routes)
//   - disagreements — model/schema disagreements (model.disagreements)
//   - unguarded    — models that explicitly wrote `$guarded = []` (Laravel's
//                    "everything is mass-assignable" escape hatch)
//
// The unguarded count keys on guarded being a non-nil empty array: an empty
// array is the risky explicit escape hatch, whereas a nil guarded (property
// omitted) is guarded-by-omission and NOT a finding. This matches the load-
// bearing nil-vs-empty distinction the Go Model documents.
export function healthVerdict(model) {
  model = model || {};

  const unguarded = (model.models || []).filter(
    (m) => Array.isArray(m.guarded) && m.guarded.length === 0
  ).length;

  // One row per category, in the fixed understand-then-judge order the verdict
  // itemizes (dead routes → disagreements → unguarded). `noun` is the singular;
  // the label pluralizes on count.
  const categories = [
    { kind: "dead_routes", count: (model.dead_routes || []).length, noun: "dead route" },
    { kind: "disagreements", count: (model.disagreements || []).length, noun: "disagreement" },
    { kind: "unguarded", count: unguarded, noun: "unguarded model" },
  ];

  const problems = categories
    .filter((c) => c.count > 0)
    .map((c) => ({
      kind: c.kind,
      count: c.count,
      label: c.count + " " + c.noun + (c.count === 1 ? "" : "s"),
      view: "findings",
    }));

  const clean = problems.length === 0;
  return { clean, problems };
}
