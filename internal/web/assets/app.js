// Un(la)ravel dashboard — single-page client.
//
// Drives the UI in index.html against the local server's JSON API:
//   GET /api/analyze?path=<local-path>  ->  { model, mermaid, openapi }
// where `model` is the unlaravel.json contract (schema_version, project_name,
// laravel_version, schemas, models, routes, controllers, dead_routes,
// form_requests, disagreements), `mermaid` is the ER diagram source, and
// `openapi` is the OpenAPI 3 document. Errors come back as { error } with a 4xx.
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
  };

  let lastRoutes = []; // for client-side filtering

  if (window.mermaid) {
    mermaid.initialize({ startOnLoad: false, theme: "dark", securityLevel: "loose" });
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
    const cards = [
      { n: (model.schemas || []).length, l: "Tables" },
      { n: (model.models || []).length, l: "Models" },
      { n: (model.controllers || []).length, l: "Controllers" },
      { n: (model.routes || []).length, l: "Routes" },
      { n: (model.form_requests || []).length, l: "Form Requests" },
      { n: dead, l: "Dead Routes", warn: dead > 0 },
      { n: disagree, l: "Disagreements", warn: disagree > 0 },
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
    if (!mermaidSrc || !window.mermaid) {
      el.erDiagram.innerHTML = '<p class="hint">No schema to diagram.</p>';
      return;
    }
    try {
      const { svg } = await mermaid.render("er-graph-" + Date.now(), mermaidSrc);
      el.erDiagram.innerHTML = svg;
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
