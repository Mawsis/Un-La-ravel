// API Docs view: Swagger UI over the generated OpenAPI spec, framed in a
// light "island" panel per design.md's deliberate trade — a full dark
// override of swagger-ui.css is out of scope for this redesign.

import { $ } from "../dom.js";

export function renderSwagger(openapi) {
  const container = $("#swagger-ui");
  if (!openapi || !window.SwaggerUIBundle) {
    container.innerHTML = '<p class="hint" style="padding:16px">No OpenAPI spec available.</p>';
    return;
  }
  // SwaggerUIBundle mutates the target; rebuild each analysis.
  container.innerHTML = "";
  SwaggerUIBundle({
    spec: openapi,
    domNode: container,
    deepLinking: false,
    presets: [SwaggerUIBundle.presets.apis],
    layout: "BaseLayout",
  });
}
