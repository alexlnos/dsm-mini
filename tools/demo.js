// Screenshots and a walkthrough recording for an article.
//
// Same idea as shoot.js — the NAS answers are replaced with fixtures, so
// nothing real ever reaches a picture — but this one walks the whole app in
// one language and also records the walk as a video, which ffmpeg turns into
// a GIF afterwards.
//
//   NODE_PATH=/tmp/pw/node_modules node tools/demo.js
//
// Result: docs/article/NN-name.png and docs/article/walkthrough.gif
const { chromium } = require('playwright');
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');

const fixtures = require('./fixtures.js');

const TOKEN = process.env.TELEGRAM_BOT_TOKEN || 'demo-token';
const BASE = process.env.BASE || 'http://127.0.0.1:8099';
const LANG = process.env.LANG_CODE || 'ru';
const OUT = path.join(__dirname, '..', 'docs', 'article');
// Playwright ships an ffmpeg with sixteen filters in it — enough to write a
// webm, not enough to build a GIF palette. A real one is needed.
const FFMPEG = process.env.FFMPEG ||
  ['/opt/homebrew/bin/ffmpeg', '/usr/local/bin/ffmpeg', '/usr/bin/ffmpeg']
    .find((f) => fs.existsSync(f));

// A picture for the file preview: the route has to answer with real bytes,
// and a photo of anyone's is exactly what must not be here.
const SAMPLE = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
  'base64');

function initData(lang) {
  const user = JSON.stringify({ id: 182957384, first_name: 'Alex', username: 'demo', language_code: lang });
  const fields = { auth_date: String(Math.floor(Date.now() / 1000)), query_id: 'AAHdemo', user };
  const check = Object.keys(fields).sort().map((k) => `${k}=${fields[k]}`).join('\n');
  const secret = crypto.createHmac('sha256', 'WebAppData').update(TOKEN).digest();
  fields.hash = crypto.createHmac('sha256', secret).update(check).digest('hex');
  return new URLSearchParams(fields).toString();
}

async function stub(ctx) {
  await ctx.route('**/api/**', (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.startsWith('/api/files/thumb') || url.pathname.startsWith('/api/files/preview')) {
      return route.fulfill({ status: 200, contentType: 'image/png', body: SAMPLE });
    }
    const body = fixtures[url.pathname];
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(body ?? {}),
    });
  });
}

const settle = (page, ms = 550) => page.waitForTimeout(ms);

// Every step is allowed to fail on its own: a changed label should cost one
// picture, not the whole run.
async function step(name, fn) {
  try {
    await fn();
    return true;
  } catch (e) {
    console.log(`  ! ${name}: ${e.message.split('\n')[0]}`);
    return false;
  }
}

(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch();

  // ── still screenshots ────────────────────────────────────────────────────
  const ctx = await browser.newContext({
    viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, locale: LANG,
  });
  await stub(ctx);
  const page = await ctx.newPage();
  await page.goto(`${BASE}/#${new URLSearchParams({ tgWebAppData: initData(LANG) })}`);
  await page.waitForSelector('.app-card', { timeout: 20000 });
  await settle(page, 900);

  let n = 0;
  const shot = async (name) => {
    n += 1;
    await settle(page, 450);
    await page.screenshot({ path: path.join(OUT, `${String(n).padStart(2, '0')}-${name}.png`) });
    console.log(`  ${String(n).padStart(2, '0')}-${name}`);
  };

  const tab = (i) => page.locator('.tab').nth(i);
  const card = (i) => page.locator('.app-card').nth(i);

  await shot('home');

  await step('downloads', async () => { await tab(1).click(); await settle(page); });
  await shot('downloads');

  await step('task', async () => { await page.locator('.task-open').first().click(); await settle(page); });
  await shot('task');

  // The files of a task are already open — what is below the fold is the
  // interesting part: per-file priority and the trackers.
  await step('task scroll', async () => {
    await page.mouse.wheel(0, 900);
    await settle(page, 700);
  });
  await shot('task-files');

  await step('add', async () => {
    await tab(1).click(); await settle(page);
    await page.getByRole('button', { name: /Добавить загрузку/ }).click();
    await settle(page);
  });
  await shot('add');

  await step('folders', async () => {
    await page.getByRole('button', { name: /Настроить/ }).click();
    await settle(page);
  });
  await shot('folders');

  await step('files', async () => { await tab(2).click(); await settle(page, 800); });
  await shot('files');

  await step('file list inside', async () => {
    await page.locator('.entry').first().click();
    await settle(page, 800);
  });
  await shot('files-inside');

  // These screens have no tab bar — they are opened from the home screen and
  // left through the back button, so the way back is a reload, not a tab.
  for (const [index, name] of [[4, 'storage'], [2, 'vms'], [3, 'containers'], [5, 'log']]) {
    await step(name, async () => {
      // A goto that only changes the fragment does not reload the page, and
      // the app would stay on the screen it was already showing.
      await page.goto(`${BASE}/?r=${n}#${new URLSearchParams({ tgWebAppData: initData(LANG) })}`);
      await page.waitForSelector('.app-card', { timeout: 20000 });
      await settle(page, 700);
      await card(index).click();
      await settle(page, 1100);
    });
    await shot(name);
  }

  await ctx.close();

  // ── the walkthrough ──────────────────────────────────────────────────────
  const rec = await browser.newContext({
    viewport: { width: 390, height: 844 }, deviceScaleFactor: 1, locale: LANG,
    recordVideo: { dir: path.join(OUT, 'video'), size: { width: 390, height: 844 } },
  });
  await stub(rec);
  const scene = await rec.newPage();
  await scene.goto(`${BASE}/#${new URLSearchParams({ tgWebAppData: initData(LANG) })}`);
  await scene.waitForSelector('.app-card', { timeout: 20000 });
  await settle(scene, 1600);

  const beat = (fn, ms = 1500) => step('scene', async () => { await fn(); await settle(scene, ms); });

  await beat(() => scene.locator('.tab').nth(1).click());
  await beat(() => scene.locator('.task-open').first().click(), 1800);
  await beat(() => scene.mouse.wheel(0, 700), 1500);
  await beat(() => scene.locator('.tab').nth(1).click());
  await beat(() => scene.getByRole('button', { name: /Добавить загрузку/ }).click(), 1200);
  await beat(async () => {
    const field = scene.locator('textarea').first();
    await field.click();
    await field.type('magnet:?xt=urn:btih:88594aaacbde40ef3e2510c47374ec0aa396c08e', { delay: 26 });
  }, 1500);
  await beat(() => scene.locator('.tab').nth(2).click(), 1600);
  await beat(() => scene.locator('.tab').nth(0).click(), 1600);

  const video = scene.video();
  await rec.close();
  const webm = await video.path();

  // A GIF wants its own palette, otherwise the gradients turn to mud.
  const gif = path.join(OUT, 'walkthrough.gif');
  const filters = 'fps=12,scale=320:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=160[p];[b][p]paletteuse=dither=bayer:bayer_scale=3';
  execFileSync(FFMPEG, ['-y', '-i', webm, '-filter_complex', filters, gif], { stdio: 'pipe' });
  fs.rmSync(path.join(OUT, 'video'), { recursive: true, force: true });

  const kb = Math.round(fs.statSync(gif).size / 1024);
  console.log(`  walkthrough.gif — ${kb} KB`);

  await browser.close();
  console.log(`${n} screenshots in ${OUT}`);
})();
