# Screenshots for the documentation

The screenshots in `docs/screenshots` are taken on **demo data**, not on a live
NAS: a README must not show someone's torrents, machine names and folders.

`fixtures.js` holds that data, `shoot.js` walks the app. The API answers are
replaced at the browser level, so no NAS is needed for the shoot — a running
service is enough.

```bash
cd tools && npm i playwright
TELEGRAM_BOT_TOKEN=... OUT=./out node shoot.js
```

The shots are then scaled down by a third and converted to WebP: 21 KB instead
of 135 KB per file across the same fifty screens.
