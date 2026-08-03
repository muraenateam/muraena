'use strict';
const puppeteer = require('puppeteer');

function arg(name, def) {
  const i = process.argv.indexOf('--' + name);
  return i >= 0 && process.argv[i + 1] ? process.argv[i + 1] : def;
}
function progress(msg) { process.stderr.write(JSON.stringify({ type: 'progress', msg }) + '\n'); }

(async () => {
  const target = arg('target');
  if (!target) { process.stderr.write('missing --target\n'); process.exit(2); }
  const depth = parseInt(arg('depth', '1'), 10);
  const maxPages = parseInt(arg('max-pages', '20'), 10);
  const timeout = parseInt(arg('timeout', '20000'), 10);

  const origins = new Set();
  const loginPages = [];
  const targetHost = new URL(target).hostname;

  const browser = await puppeteer.launch({
    headless: 'new',
    args: ['--no-sandbox'],
    ignoreHTTPSErrors: true,
  });
  const queue = [{ url: target, d: 0 }];
  const seen = new Set();
  let visited = 0;

  while (queue.length && visited < maxPages) {
    const { url, d } = queue.shift();
    if (seen.has(url)) continue;
    seen.add(url);
    visited++;
    progress(`visiting ${url} (${visited}/${maxPages})`);

    const page = await browser.newPage();
    page.on('request', (req) => {
      try { origins.add(new URL(req.url()).hostname); } catch (_) {}
    });
    try {
      await page.goto(url, { waitUntil: 'networkidle2', timeout });

      // detect login forms
      const forms = await page.evaluate(() => {
        const out = [];
        document.querySelectorAll('form').forEach((f) => {
          const pw = f.querySelector('input[type=password]');
          if (!pw) return;
          const user = f.querySelector('input[type=email], input[name*=user i], input[type=text]');
          out.push({
            action: f.getAttribute('action') || location.pathname,
            usernameSelector: user ? (user.id ? '#' + user.id : user.name ? `[name="${user.name}"]` : 'input[type=text]') : '',
            passwordSelector: pw.id ? '#' + pw.id : pw.name ? `[name="${pw.name}"]` : 'input[type=password]',
          });
        });
        return out;
      });
      forms.forEach((fm) => loginPages.push(Object.assign({ url }, fm)));

      // enqueue same-host links
      if (d < depth) {
        const links = await page.evaluate(() => Array.from(document.querySelectorAll('a[href]')).map((a) => a.href));
        links.forEach((l) => {
          try { if (new URL(l).hostname === targetHost && !seen.has(l)) queue.push({ url: l, d: d + 1 }); } catch (_) {}
        });
      }
    } catch (e) {
      progress(`error at ${url}: ${e.message}`);
    } finally {
      await page.close();
    }
  }
  await browser.close();

  const secretsPaths = [...new Set(loginPages.map((l) => l.action).filter(Boolean))];
  const secretsPatterns = [
    { label: 'password', matching: 'password', start: '', end: '' },
    { label: 'username', matching: 'email', start: '', end: '' },
  ];

  process.stdout.write(JSON.stringify({
    target: targetHost,
    origins: [...origins].filter((h) => h && h !== targetHost),
    loginPages, secretsPaths, secretsPatterns,
  }));
  process.exit(0);
})();
