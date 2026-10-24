import { createRequire } from 'node:module';
import { chromium } from 'playwright';
import { extractArticle } from './extract.mjs';

const require = createRequire(import.meta.url);
const sourceURL = process.argv[2];
const allowedHost = (host) => ['openai.com', 'oaistatic.com', 'ctfassets.net', 'oaiusercontent.com'].some(domain => host === domain || host.endsWith(`.${domain}`));
function articleURL(raw) {
  const url = new URL(raw);
  return url.protocol === 'https:' && !url.username && !url.password && !url.port &&
    ['openai.com', 'www.openai.com'].includes(url.hostname) && url.pathname.startsWith('/index/');
}
let browser;
process.on('SIGTERM', async () => {
  await browser?.close().catch(() => {});
  process.exit(1);
});
try {
  if (!articleURL(sourceURL)) throw new Error('Unsupported article URL');
  // Public pages load reliably in normal Chrome. No logged-in profile is used.
  browser = await chromium.launch({ channel: 'chrome', headless: false });
  const context = await browser.newContext({ viewport: { width: 1200, height: 900 }, locale: 'en-US', serviceWorkers: 'block' });
  const page = await context.newPage();
  await page.route('**/*', async route => {
    const request = route.request();
    const url = new URL(request.url());
    const mainNavigation = request.isNavigationRequest() && request.frame() === page.mainFrame();
    if (url.protocol !== 'https:' || !allowedHost(url.hostname) || url.port ||
      (mainNavigation && !articleURL(url.href))) return route.abort();
    if (['media', 'font'].includes(request.resourceType())) return route.abort();
    return route.continue();
  });
  const response = await page.goto(sourceURL, { waitUntil: 'domcontentloaded', timeout: 60000 });
  if (!response?.ok() || !articleURL(page.url())) throw new Error(`Article request failed (${response?.status() ?? 0})`);
  await page.locator('main p').first().waitFor({ timeout: 15000 });
  await page.addScriptTag({ path: require.resolve('@mozilla/readability/Readability.js') });
  const result = await page.evaluate(extractArticle);
  process.stdout.write(JSON.stringify(result));
} catch (error) {
  process.stderr.write(`${error.message}\n`);
  process.exitCode = 1;
} finally {
  await browser?.close();
}
