// Un(la)ravel dashboard — single-page client.
//
// Drives the UI in index.html against the local server's JSON API:
//   GET /api/analyze?path=<local-path>  ->  { model, mermaid, openapi }
// where `model` is the unlaravel.json contract (schema_version, project_name,
// laravel_version, schemas, models, routes, controllers, dead_routes,
// form_requests, disagreements), `mermaid` is the ER diagram source, and
// `openapi` is the OpenAPI 3 document. Errors come back as { error } with a 4xx.
//
// Per-model fillable/guarded/casts and per-table indexes (schemas[].indexes)
// are rendered on the Models tab; see renderModels.
//
// No framework, no build step. Loaded from //go:embed inside the Go binary.

(function () {
  "use strict";

  const $ = (sel) => document.querySelector(sel);

  const el = {
    form: $("#search-form"),
    input: $("#path-input"),
    btn: $("#analyze-btn"),
    intro: $("#intro"),
    status: $("#status"),
    results: $("#results"),
    projMeta: $("#proj-meta"),
    cards: $("#cards"),
    erDiagram: $("#er-diagram"),
    routesBody: $("#routes-body"),
    routeFilter: $("#route-filter"),
    findingsBody: $("#findings-body"),
    badgeRoutes: $("#badge-routes"),
    badgeFindings: $("#badge-findings"),
    swagger: $("#swagger-ui"),
    modelsBody: $("#models-body"),
    modelFilter: $("#model-filter"),
    badgeModels: $("#badge-models"),
  };

  let lastRoutes = []; // for client-side filtering
  let erPanZoom = null; // svg-pan-zoom instance for the current ER diagram

  if (window.mermaid) {
    mermaid.initialize({
      startOnLoad: false,
      theme: "dark",
      securityLevel: "loose",
      // useMaxWidth:false stops Mermaid shrinking a large diagram to the
      // container width (which made big schemas unreadable); the SVG renders
      // at natural size and svg-pan-zoom provides navigation instead.
      er: { useMaxWidth: false },
    });
  }

  // ---- data fetch -----------------------------------------------------------

  async function analyze(path) {
    setLoading(true);
    try {
      const res = await fetch("/api/analyze?path=" + encodeURIComponent(path));
      const body = await res.json();
      if (!res.ok) {
        throw new Error(body && body.error ? body.error : "Analysis failed (HTTP " + res.status + ")");
      }
      render(body);
    } catch (err) {
      showError(err && err.message ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }

  function setLoading(on) {
    el.btn.disabled = on;
    if (on) {
      el.status.innerHTML = '<div class="loading"><span class="spinner"></span>Analyzing…</div>';
    } else {
      el.status.innerHTML = "";
    }
  }

  function showError(msg) {
    el.results.classList.add("hidden");
    el.status.innerHTML = '<div class="error">⚠️ ' + escapeHtml(msg) + "</div>";
  }

  // ---- rendering ------------------------------------------------------------

  function render(data) {
    const model = data.model || {};
    el.intro.classList.add("hidden");
    el.results.classList.remove("hidden");

    renderMeta(model);
    renderCards(model);
    renderER(data.mermaid);
    renderModels(model.models || [], model.schemas || []);
    renderRoutes(model.routes || [], model.dead_routes || []);
    renderFindings(model.disagreements || [], model.dead_routes || []);
    renderSwagger(data.openapi);

    // reset to first tab
    activateTab("er");
  }

  function renderMeta(model) {
    const name = model.project_name || "(unnamed project)";
    const ver = model.laravel_version || "?";
    el.projMeta.innerHTML =
      "Project <strong>" + escapeHtml(name) + "</strong> · Laravel <strong>" +
      escapeHtml(ver) + "</strong> · schema " + escapeHtml(model.schema_version || "?");
  }

  function renderCards(model) {
    const dead = (model.dead_routes || []).length;
    const disagree = (model.disagreements || []).length;
    const unguardedCount = (model.models || []).filter(
      (m) => Array.isArray(m.guarded) && m.guarded.length === 0
    ).length;
    const cards = [
      { n: (model.schemas || []).length, l: "Tables" },
      { n: (model.models || []).length, l: "Models" },
      { n: (model.controllers || []).length, l: "Controllers" },
      { n: (model.routes || []).length, l: "Routes" },
      { n: (model.form_requests || []).length, l: "Form Requests" },
      { n: dead, l: "Dead Routes", warn: dead > 0 },
      { n: disagree, l: "Disagreements", warn: disagree > 0 },
      { n: unguardedCount, l: "Unguarded", warn: unguardedCount > 0 },
    ];
    el.cards.innerHTML = cards
      .map(
        (c) =>
          '<div class="card' + (c.warn ? " warn" : "") + '"><div class="n">' +
          c.n + '</div><div class="l">' + c.l + "</div></div>"
      )
      .join("");
  }

  async function renderER(mermaidSrc) {
    // Tear down the previous pan-zoom instance before replacing the SVG,
    // otherwise its listeners leak across analyses.
    if (erPanZoom) {
      erPanZoom.destroy();
      erPanZoom = null;
    }
    if (!mermaidSrc || !window.mermaid) {
      el.erDiagram.innerHTML = '<p class="hint">No schema to diagram.</p>';
      return;
    }
    try {
      const { svg } = await mermaid.render("er-graph-" + Date.now(), mermaidSrc);
      el.erDiagram.innerHTML = svg;
      const svgEl = el.erDiagram.querySelector("svg");
      if (svgEl && window.svgPanZoom) {
        // Fill the fixed-height container; pan-zoom drives the viewport.
        svgEl.style.width = "100%";
        svgEl.style.height = "100%";
        svgEl.style.maxWidth = "none";
        erPanZoom = svgPanZoom(svgEl, {
          zoomEnabled: true,
          panEnabled: true,
          mouseWheelZoomEnabled: true,
          dblClickZoomEnabled: true,
          controlIconsEnabled: true,
          fit: true,
          center: true,
          minZoom: 0.1,
          maxZoom: 50,
          zoomScaleSensitivity: 0.35,
        });
      }
    } catch (e) {
      // Fall back to showing the source if Mermaid can't parse it.
      el.erDiagram.innerHTML =
        '<p class="hint">Could not render the diagram; showing source:</p><pre>' +
        escapeHtml(mermaidSrc) + "</pre>";
    }
  }

  function deadKey(r) {
    return [r.method, r.uri, r.controller, r.action].join("|");
  }

  function renderRoutes(routes, deadRoutes) {
    lastRoutes = routes;
    const deadSet = new Set((deadRoutes || []).map(deadKey));
    el.badgeRoutes.textContent = routes.length;
    drawRouteRows(routes, deadSet, "");
    el.routeFilter.oninput = () => drawRouteRows(routes, deadSet, el.routeFilter.value);
  }

  function drawRouteRows(routes, deadSet, filter) {
    const q = filter.trim().toLowerCase();
    const rows = routes
      .filter((r) => {
        if (!q) return true;
        return (
          (r.method || "").toLowerCase().includes(q) ||
          (r.uri || "").toLowerCase().includes(q) ||
          (r.controller || "").toLowerCase().includes(q) ||
          (r.action || "").toLowerCase().includes(q)
        );
      })
      .map((r) => {
        const isDead = deadSet.has(deadKey(r));
        const action =
          r.controller || r.action
            ? escapeHtml((r.controller || "—") + "@" + (r.action || "—"))
            : '<span class="mw">(closure / view route)</span>';
        const mw = (r.middleware || []).length
          ? '<span class="mw">' + escapeHtml(r.middleware.join(", ")) + "</span>"
          : '<span class="mw">—</span>';
        return (
          '<tr class="' + (isDead ? "dead" : "") + '">' +
          '<td class="method m-' + escapeHtml(r.method || "") + '">' + escapeHtml(r.method || "") + "</td>" +
          '<td class="uri">' + escapeHtml(r.uri || "") + (isDead ? '<span class="dead-tag">DEAD</span>' : "") + "</td>" +
          '<td class="action">' + action + "</td>" +
          "<td>" + mw + "</td>" +
          "</tr>"
        );
      })
      .join("");
    el.routesBody.innerHTML = rows || '<tr><td colspan="4" class="hint" style="padding:16px">No routes match.</td></tr>';
  }

  // ---- models -----------------------------------------------------------

  function renderModels(models, schemas) {
    const unguardedCount = models.filter(
      (m) => Array.isArray(m.guarded) && m.guarded.length === 0
    ).length;
    el.badgeModels.textContent = models.length;
    el.badgeModels.classList.toggle("danger", unguardedCount > 0);

    drawModelCards(models, schemas, "");
    el.modelFilter.oninput = () => drawModelCards(models, schemas, el.modelFilter.value);
  }

  // massAssignmentState classifies a model's fillable/guarded pair into one
  // of four states. null vs. [] is load-bearing here (see the doc comment on
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
        .map(
          (c) =>
            '<span class="chip">' + escapeHtml(c.column || "") + ": " + escapeHtml(c.type || "") + "</span>"
        )
        .join("") +
      "</div>"
    );
  }

  // renderIndexesSection renders the indexes for a table (found or not).
  // tableName is only used for the "table not found" / labeled-header cases.
  function renderIndexesSection(table, tableName) {
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
      indexes
        .filter((idx) => (idx.columns || []).length > 0)
        .map((idx) => idx.columns[0])
    );
    const columns = table.columns || [];
    const hints = columns
      .filter((c) => c.is_foreign_key === true && c.is_primary_key !== true)
      .filter((c) => !covered.has(c.name))
      .map(
        (c) =>
          '<div class="fk-hint">FK ' + escapeHtml(c.name) + " has no covering index in migrations</div>"
      )
      .join("");
    return hints;
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
          renderIndexesSection(table, m.table) +
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
            renderIndexesSection(t, t.name) +
            renderFkHints(t) +
            "</div>"
          );
        })
        .join("");
    }

    const html = modelCards + orphanHtml;
    el.modelsBody.innerHTML = html || '<div class="hint" style="padding:16px">No models or tables match.</div>';
  }

  function renderFindings(disagreements, deadRoutes) {
    const total = (disagreements || []).length + (deadRoutes || []).length;
    el.badgeFindings.textContent = total;
    el.badgeFindings.classList.toggle("danger", total > 0);

    if (total === 0) {
      el.findingsBody.innerHTML = '<div class="all-clear">✅ No dead routes, no Model↔Schema disagreements. All clear.</div>';
      return;
    }

    let html = "";
    (deadRoutes || []).forEach((d) => {
      html +=
        '<div class="finding dead"><div class="h">⚠️ Dead route: ' +
        escapeHtml((d.method || "") + " " + (d.uri || "")) + "</div>" +
        '<div class="r">' + escapeHtml(d.reason || d.kind || "") + "</div></div>";
    });
    (disagreements || []).forEach((d) => {
      html +=
        '<div class="finding"><div class="h">⚠️ Disagreement: ' +
        escapeHtml((d.model || "") + "::" + (d.relationship || "")) + "</div>" +
        '<div class="r">' + escapeHtml(d.reason || d.kind || "") + "</div></div>";
    });
    el.findingsBody.innerHTML = html;
  }

  let swaggerRendered = null;
  function renderSwagger(openapi) {
    if (!openapi || !window.SwaggerUIBundle) {
      el.swagger.innerHTML = '<p class="hint" style="padding:16px">No OpenAPI spec available.</p>';
      return;
    }
    // SwaggerUIBundle mutates the target; rebuild each analysis.
    el.swagger.innerHTML = "";
    swaggerRendered = SwaggerUIBundle({
      spec: openapi,
      domNode: el.swagger,
      deepLinking: false,
      presets: [SwaggerUIBundle.presets.apis],
      layout: "BaseLayout",
    });
  }

  // ---- tabs -----------------------------------------------------------------

  function activateTab(name) {
    document.querySelectorAll(".tab").forEach((t) =>
      t.classList.toggle("active", t.dataset.tab === name)
    );
    document.querySelectorAll(".panel").forEach((p) =>
      p.classList.toggle("active", p.dataset.panel === name)
    );
  }

  document.querySelectorAll(".tab").forEach((t) =>
    t.addEventListener("click", () => activateTab(t.dataset.tab))
  );

  // ---- helpers --------------------------------------------------------------

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  // ---- wire up --------------------------------------------------------------

  el.form.addEventListener("submit", (e) => {
    e.preventDefault();
    const path = el.input.value.trim();
    if (path) analyze(path);
  });
})();
