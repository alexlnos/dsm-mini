// Captures the app screens in every language on demo data.
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const fixtures = require('./fixtures');

const TOKEN = process.env.TELEGRAM_BOT_TOKEN;
const BASE = 'http://127.0.0.1:8099';
const OUT = process.env.OUT || path.join(__dirname, 'out');
const LANGS = ['en', 'ru', 'es', 'pt', 'de', 'fr', 'it', 'tr', 'uk', 'pl'];

function initData(lang) {
  const user = JSON.stringify({ id: 182957384, first_name: 'Alex', username: 'demo', language_code: lang });
  const fields = { auth_date: String(Math.floor(Date.now() / 1000)), query_id: 'AAHdemo', user };
  const check = Object.keys(fields).sort().map((k) => `${k}=${fields[k]}`).join('\n');
  const secret = crypto.createHmac('sha256', 'WebAppData').update(TOKEN).digest();
  fields.hash = crypto.createHmac('sha256', secret).update(check).digest('hex');
  return new URLSearchParams(fields).toString();
}

(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch();

  for (const lang of LANGS) {
    const ctx = await browser.newContext({
      viewport: { width: 390, height: 844 },
      deviceScaleFactor: 2,
      locale: lang,
    });

    // Replace the NAS answers: nothing real may end up in a screenshot.
    await ctx.route('**/api/**', (route) => {
      const url = new URL(route.request().url());
      const body = fixtures[url.pathname];
      if (!body) return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
    });

    const page = await ctx.newPage();
    await page.goto(`${BASE}/#${new URLSearchParams({ tgWebAppData: initData(lang) })}`);
    await page.waitForSelector('.app-card', { timeout: 15000 });
    await page.waitForTimeout(700);

    const shot = async (name) => {
      await page.waitForTimeout(450);
      await page.screenshot({ path: path.join(OUT, `${lang}-${name}.png`) });
    };

    await shot('home');

    await page.locator('.tab').nth(1).click();           // Downloads
    await shot('downloads');

    await page.locator('.task-open').first().click();    // task details
    await shot('task');

    await page.locator('.tab').nth(2).click();           // Files
    await shot('files');

    await page.locator('.tab').nth(0).click();           // Home
    await page.waitForTimeout(400);
    await page.locator('.app-card').nth(4).click();      // Storage
    await shot('storage');

    await ctx.close();
    console.log(lang, 'done');
  }

  await browser.close();
})();
