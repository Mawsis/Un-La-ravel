// The interactive layer over the Overview constellation (issue #52). The wow
// choreography and the seeded point geometry (overview-constellation.js) are
// unchanged; this module only adds identity and interaction to the resolved
// end-state: it binds each drawn dot to a real entity, hit-tests a pointer to
// the nearest dot, and builds the visually-hidden entity list that gives the
// map a keyboard/assistive-technology path so it is never mouse-only.
//
// No DOM, no canvas here — pure mapping + geometry, so every function is
// unit-testable and can't drift from the choreography that consumes it (the
// same split overview-constellation.js keeps from overview-wow.js).

import { KIND_KEYS } from "./overview-constellation.js";
import { entityChip } from "../chip.js";
import { escapeHtml } from "../dom.js";

// assignEntities binds each drawn dot to one real entity OF ITS OWN KIND. The
// constellation subsamples entities to a node budget, so a kind with 500 tables
// draws only a handful of dots; this spreads that kind's real name list evenly
// across its drawn dots by proportional index, in source order. Pure and
// index-based (no RNG), so the binding is deterministic — the same project
// always names the same dot the same entity, preserving the constellation's
// determinism (a hard convention, CLAUDE.md).
//
// entities is { tables, models, controllers, routes, requests }: each a list of
// display names in source/discovery order (KIND_KEYS order mirrors the node
// kind indices). A node whose kind has no names is left unassigned (entityName
// null) rather than mis-bound to another kind.
export function assignEntities(nodes, entities) {
  const perKind = KIND_KEYS.map((k) => (entities && entities[k]) || []);
  // Count the drawn dots of each kind so the spread is proportional per kind.
  const drawnOfKind = KIND_KEYS.map(
    (_, ki) => nodes.reduce((acc, n) => (n.kind === ki ? acc + 1 : acc), 0)
  );
  const seenOfKind = KIND_KEYS.map(() => 0);

  return nodes.map((n) => {
    const names = perKind[n.kind];
    if (!names.length) return { ...n, entityName: null, entityKind: KIND_KEYS[n.kind] };
    const i = seenOfKind[n.kind]++;
    const drawn = drawnOfKind[n.kind];
    // Map the i-th drawn dot of this kind onto the name list proportionally, so
    // the sampled dots span the whole list rather than clustering at its head.
    const nameIdx = drawn <= 1 ? 0 : Math.min(names.length - 1, Math.floor((i / drawn) * names.length));
    return { ...n, entityName: names[nameIdx], entityKind: KIND_KEYS[n.kind] };
  });
}

// HIT_RADIUS is the pointer tolerance (px) around a dot's settled centre. Dots
// are tiny (size ~2-3px), so a generous radius makes them practical targets
// without dots so far apart that the nearest is ambiguous.
export const HIT_RADIUS = 14;

// hitTest maps a pointer position (x, y in canvas CSS pixels) onto the nearest
// drawn dot within HIT_RADIUS of its settled centre (tx, ty), or null when the
// pointer is over empty sky. Interaction begins only after the settle, so the
// resolved end-state coordinates (tx, ty) are the hit targets — never the
// in-flight positions. Pure (no DOM) so it is unit-testable without a canvas.
export function hitTest(nodes, x, y) {
  let best = null;
  let bestD2 = HIT_RADIUS * HIT_RADIUS;
  for (const n of nodes) {
    const dx = n.tx - x;
    const dy = n.ty - y;
    const d2 = dx * dx + dy * dy;
    if (d2 <= bestD2) {
      bestD2 = d2;
      best = n;
    }
  }
  return best;
}

// entityListHtml builds the visually-hidden, focusable, linked list that gives
// the constellation an equivalent keyboard/assistive-technology path — the map
// is never mouse-only. Each entity is a list item grouped under its kind; a
// model is rendered through the shared entityChip so its detail-page link can
// never drift from the rest of the app (the delegated router handler in main.js
// navigates it exactly like any chip). Other kinds are named as plain text
// (only models have a detail page today, mirroring chipTarget). Pure (the whole
// entity set in, one markup string out) so the AT contract is unit-testable.
export function entityListHtml(entities) {
  const groups = KIND_KEYS.map((k) => {
    const names = (entities && entities[k]) || [];
    if (!names.length) return "";
    const items = names
      .map((name) => {
        const cell =
          k === "models"
            ? entityChip({ kind: "model", name })
            : escapeHtml(name);
        return "<li>" + cell + "</li>";
      })
      .join("");
    return (
      '<li><span class="entity-group-label">' + escapeHtml(KIND_LABEL[k]) + "</span>" +
      "<ul>" + items + "</ul></li>"
    );
  }).join("");

  // The list lives inside a .sr-only host (index.html), so the entity-group-label
  // and constellation-entities classes are structural hooks — they carry no CSS
  // today (the subtree is visually hidden) but name the DOM so a future visible
  // "list view" of the constellation, or AT-specific styling, has anchors to hang
  // on without re-deriving them.
  return (
    '<h3 class="sr-only">Entities in this project</h3>' +
    '<ul class="constellation-entities">' + groups + "</ul>"
  );
}

// KIND_LABEL gives each kind a human heading for the AT list, in KIND_KEYS
// order so the list reads in the same order the constellation clusters them.
const KIND_LABEL = {
  tables: "Tables",
  models: "Models",
  controllers: "Controllers",
  routes: "Routes",
  requests: "Form requests",
};
