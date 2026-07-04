// Models view: mass-assignment state, casts, indexes, orphan tables. Ported
// from app.js's renderModels/drawModelCards and helpers with no behavior
// change.

import { $, escapeHtml } from "../dom.js";

export function renderModels(models, schemas) {
  const unguardedCount = models.filter(
    (m) => Array.isArray(m.guarded) && m.guarded.length === 0
  ).length;

  const badge = $("#badge-models");
  badge.textContent = models.length;
  badge.classList.toggle("danger", unguardedCount > 0);

  drawModelCards(models, schemas, "");
  $("#model-filter").oninput = (e) => drawModelCards(models, schemas, e.target.value);
}

// massAssignmentState classifies a model's fillable/guarded pair into one of
// four states. null vs. [] is load-bearing here (see the doc comment on
// Fillable/Guarded in internal/model/eloquent.go): a bare "|| []" coercion
// would erase the distinction between "declared empty" and "not declared".
function massAssignmentState(m) {
  if (Array.isArray(m.guarded) && m.guarded.length === 0) {
    return "unguarded";
  }
  if (Array.isArray(m.fillable)) {
    return "fillable";
  }
  if (Array.isArray(m.guarded)) {
    return "guarded";
  }
  return "protected";
}

function renderMassAssignmentSection(m) {
  const state = massAssignmentState(m);
  if (state === "unguarded") {
    return (
      '<span class="pill danger">Unguarded</span>' +
      '<div class="empty-note">$guarded = [] — every column is mass-assignable</div>'
    );
  }
  if (state === "fillable") {
    const cols = m.fillable;
    if (cols.length === 0) {
      return (
        '<span class="pill ok">Fillable (0)</span>' +
        '<div class="empty-note">declared empty — nothing mass-assignable</div>'
      );
    }
    return (
      '<span class="pill ok">Fillable (' + cols.length + ")</span>" +
      '<div class="chips">' + cols.map((c) => '<span class="chip">' + escapeHtml(c) + "</span>").join("") + "</div>"
    );
  }
  if (state === "guarded") {
    const cols = m.guarded;
    return (
      '<span class="pill warn">Guarded (' + cols.length + ")</span>" +
      '<div class="chips">' + cols.map((c) => '<span class="chip">' + escapeHtml(c) + "</span>").join("") + "</div>"
    );
  }
  return (
    '<span class="pill dim">Protected</span>' +
    '<div class="empty-note">no $fillable or $guarded declared — mass assignment fully guarded</div>'
  );
}

function renderCastsSection(casts) {
  const list = casts || [];
  if (list.length === 0) {
    return '<div class="subhead">CASTS</div><div class="empty-note">No casts declared</div>';
  }
  return (
    '<div class="subhead">CASTS</div><div class="chips">' +
    list
      .map((c) => '<span class="chip">' + escapeHtml(c.column || "") + ": " + escapeHtml(c.type || "") + "</span>")
      .join("") +
    "</div>"
  );
}

// renderIndexesSection renders the indexes for a table (found or not).
function renderIndexesSection(table) {
  if (!table) {
    return '<div class="subhead">INDEXES</div><div class="empty-note">Table not found in schema</div>';
  }
  const indexes = table.indexes || [];
  if (indexes.length === 0) {
    return (
      '<div class="subhead">INDEXES (' + escapeHtml(table.name) + ')</div>' +
      '<div class="empty-note">No indexes declared</div>'
    );
  }
  const rows = indexes
    .map((idx) => {
      const kindClass = idx.unique ? "unique" : "plain";
      const kindLabel = idx.unique ? "UNIQUE" : "INDEX";
      const name = idx.name ? escapeHtml(idx.name) + " " : "";
      const cols = (idx.columns || []).map((c) => escapeHtml(c)).join(", ");
      return (
        '<div class="idx-row"><span class="idx-kind ' + kindClass + '">' + kindLabel + "</span>" +
        name + '<span class="cols">(' + cols + ")</span></div>"
      );
    })
    .join("");
  return '<div class="subhead">INDEXES (' + escapeHtml(table.name) + ')</div>' + rows;
}

// renderFkHints flags foreign-key columns (excluding primary keys) with no
// covering index, i.e. no index whose leftmost column is that FK. This is a
// soft, disposable hint local to the card — not a Finding, no badge impact.
function renderFkHints(table) {
  if (!table) return "";
  const indexes = table.indexes || [];
  const covered = new Set(
    indexes.filter((idx) => (idx.columns || []).length > 0).map((idx) => idx.columns[0])
  );
  const columns = table.columns || [];
  return columns
    .filter((c) => c.is_foreign_key === true && c.is_primary_key !== true)
    .filter((c) => !covered.has(c.name))
    .map((c) => '<div class="fk-hint">FK ' + escapeHtml(c.name) + " has no covering index in migrations</div>")
    .join("");
}

function drawModelCards(models, schemas, filter) {
  const q = filter.trim().toLowerCase();
  const matches = (name, table) => {
    if (!q) return true;
    return (name || "").toLowerCase().includes(q) || (table || "").toLowerCase().includes(q);
  };

  const claimedTables = new Set(models.map((m) => m.table).filter(Boolean));

  const modelCards = models
    .filter((m) => matches(m.name, m.table))
    .map((m) => {
      const state = massAssignmentState(m);
      const table = (schemas || []).find((t) => t.name === m.table);
      const tableSuffix = m.table ? escapeHtml(m.table) : "(unknown table)";
      return (
        '<div class="model-card' + (state === "unguarded" ? " risk" : "") + '">' +
        '<div class="mh">' + escapeHtml(m.name || "") + ' <span class="mw">-&gt; ' + tableSuffix + "</span></div>" +
        renderMassAssignmentSection(m) +
        renderCastsSection(m.casts) +
        renderIndexesSection(table) +
        renderFkHints(table) +
        "</div>"
      );
    })
    .join("");

  const orphanTables = (schemas || []).filter((t) => !claimedTables.has(t.name)).filter((t) => matches("", t.name));
  let orphanHtml = "";
  if (orphanTables.length > 0) {
    orphanHtml += '<div class="subhead">TABLES WITHOUT MODELS (' + orphanTables.length + ")</div>";
    orphanHtml += orphanTables
      .map((t) => {
        const colCount = (t.columns || []).length;
        return (
          '<div class="model-card">' +
          '<div class="mh">' + escapeHtml(t.name) + "</div>" +
          '<div class="empty-note">' + colCount + " column" + (colCount === 1 ? "" : "s") + "</div>" +
          renderIndexesSection(t) +
          renderFkHints(t) +
          "</div>"
        );
      })
      .join("");
  }

  const html = modelCards + orphanHtml;
  $("#models-body").innerHTML = html || '<div class="hint" style="padding:16px">No models or tables match.</div>';
}
