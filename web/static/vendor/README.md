# Vendored front-end libraries

Served from this origin instead of a CDN, so the Content-Security-Policy can
allow `'self'` only. Each file is copied unchanged from its npm package; check
one with `sha256sum` against the package's tarball.

| File | Package | Version | License |
|---|---|---|---|
| `xterm.js`, `xterm.css` | `@xterm/xterm` (`lib/`, `css/`) | 6.0.0 | MIT |
| `addon-fit.js` | `@xterm/addon-fit` (`lib/`) | 0.11.0 | MIT |
| `addon-web-links.js` | `@xterm/addon-web-links` (`lib/`) | 0.12.0 | MIT |
| `topojson-client.min.js` | `topojson-client` (`dist/`) | 3.1.0 | ISC |
| `land-110m.json` | `world-atlas` (Natural Earth land, 1:110m) | 2.0.2 | ISC (data: public domain) |

To update one: download the tarball (`npm pack <package>@<version>`), copy the
file over, replace its `LICENSE.<package>-<version>`, and update this table.
The source maps the scripts point to aren't vendored.

```
14903579ff54664cd72f8e8699e6961a6272c21863ec1c3b118cdc8af5d4a972  xterm.js
854a7c0fb70e8b1a083c16797ab827299fb18744f5ad34f227b48337e33293c6  xterm.css
ba3ea256ce0620a0992a197d6c9baea64823fc93d8da07a9e366ca9943c18527  addon-fit.js
b74864125ef3889753b94e61eb4dabb99b2a27f920f0c9a7bd53cbe6d10f032a  addon-web-links.js
25cd02ae486cc5063e0215a4e4cfb15de83700c87ac48bac4d57dc6aaf3ebb89  topojson-client.min.js
ead5f68119c49a9250902e7da303bcb209341bbb8fefe7369a439b48b704658a  land-110m.json
```
