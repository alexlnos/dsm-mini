# Screenshots for the documentation

The screenshots in `docs/screenshots` are taken on **demo data**, not on a live
NAS: a README must not show someone's torrents, machine names and folders.

`fixtures.js` holds that data, `shoot.js` walks the app. Playwright itself is
a dev dependency of `web/`, hence NODE_PATH: the script lives in `tools/` and
would not find it otherwise. The API answers are
replaced at the browser level, so no NAS is needed for the shoot — a running
service is enough.

```bash
cd web && npx playwright install chromium      # once: the browser itself
cd .. && set -a && . ./.env && set +a
NODE_PATH=$PWD/web/node_modules OUT=/tmp/shots node tools/shoot.js
```

The shots are then scaled down by a third and converted to WebP: 21 KB instead
of 135 KB per file across the same fifty screens.

The shoot writes PNGs at 780×1688 (a 390×844 viewport at 2×). The repository
keeps WebP at 520×1125 — the size the READMEs are laid out for:

```bash
for f in /tmp/shots/*.png; do
  sips -Z 1125 "$f" --out "/tmp/r.png" >/dev/null
  cwebp -quiet -q 78 /tmp/r.png -o "docs/screenshots/$(basename "${f%.png}").webp"
done
python3 tools/make-store.py     # the wide slides are built from the English ones
```
