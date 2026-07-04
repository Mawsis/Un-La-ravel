# Vendored dashboard assets

These files are downloaded, unmodified, from npm/jsdelivr and embedded via
`//go:embed` so `unlaravel serve` works fully offline (no CDN dependency at
runtime). Each is pinned to an exact version; upstream license text is in
`licenses/`.

| File | Package | Pinned version |
|---|---|---|
| `elk.bundled.js` | [elkjs](https://www.npmjs.com/package/elkjs) | 0.9.3 |
| `svg-pan-zoom.min.js` | [svg-pan-zoom](https://www.npmjs.com/package/svg-pan-zoom) | 3.6.2 |
| `swagger-ui-bundle.js`, `swagger-ui.css` | [swagger-ui-dist](https://www.npmjs.com/package/swagger-ui-dist) | 5.32.8 |

To upgrade a pinned version, re-download the file from jsdelivr at the new
version tag, refresh the corresponding `licenses/*.LICENSE` file, and update
the table above.
