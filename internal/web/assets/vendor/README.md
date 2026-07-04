# Vendored dashboard assets

These files are downloaded, unmodified, from npm/jsdelivr and embedded via
`//go:embed` so `unlaravel serve` works fully offline (no CDN dependency at
runtime). Each is pinned to an exact version; upstream license text is in
`licenses/`.

| File | Package | Pinned version |
|---|---|---|
| `mermaid.min.js` | [mermaid](https://www.npmjs.com/package/mermaid) | 10.9.6 |
| `svg-pan-zoom.min.js` | [svg-pan-zoom](https://www.npmjs.com/package/svg-pan-zoom) | 3.6.2 |
| `swagger-ui-bundle.js`, `swagger-ui.css` | [swagger-ui-dist](https://www.npmjs.com/package/swagger-ui-dist) | 5.32.8 |
| `fonts/instrument-sans-latin-wght-normal.woff2` | [@fontsource-variable/instrument-sans](https://www.npmjs.com/package/@fontsource-variable/instrument-sans) | 5.2.8 |
| `fonts/jetbrains-mono-latin-wght-normal.woff2` | [@fontsource-variable/jetbrains-mono](https://www.npmjs.com/package/@fontsource-variable/jetbrains-mono) | 5.2.8 |

To upgrade a pinned version, re-download the file from jsdelivr at the new
version tag, refresh the corresponding `licenses/*.LICENSE` file, and update
the table above.
