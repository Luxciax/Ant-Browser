const fs = require('fs');
const path = require('path');
const readline = require('readline');
const { normalizeTimeout, normalizePathUnderRoot, toSerializable } = require('./runner_shared.cjs');
const { createBrowserGateway } = require('./runner_browser.cjs');

const DEFAULT_TIMEOUT_MS = 30000;
const DEFAULT_SNAPSHOT_LIMIT = 200;

function collectInteractiveElements(options) {
  const limit = options && options.limit ? options.limit : DEFAULT_SNAPSHOT_LIMIT;
  const query = 'a,button,input,select,textarea,summary,[role=button],[role=link],[role=checkbox],[role=radio],[role=tab],[role=menuitem],[contenteditable=""],[contenteditable=true],[onclick]';
  const escapeCSS = (value) => (window.CSS && CSS.escape ? CSS.escape(value) : String(value).replace(/[^a-zA-Z0-9_-]/g, (ch) => `\\${ch}`));
  const visible = (el) => {
    const rect = el.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) return false;
    const style = getComputedStyle(el);
    return style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0';
  };
  const selectorFor = (el) => {
    if (el.id) {
      const candidate = `#${escapeCSS(el.id)}`;
      if (document.querySelectorAll(candidate).length === 1) return candidate;
    }
    for (const attr of ['data-testid', 'data-test-id', 'name', 'aria-label']) {
      const value = el.getAttribute(attr);
      if (!value) continue;
      const candidate = `${el.tagName.toLowerCase()}[${attr}="${String(value).replace(/"/g, '\\"')}"]`;
      try {
        if (document.querySelectorAll(candidate).length === 1) return candidate;
      } catch {}
    }
    const parts = [];
    let node = el;
    while (node && node.nodeType === 1 && parts.length < 6) {
      const tag = node.tagName.toLowerCase();
      if (tag === 'html' || tag === 'body') break;
      const parent = node.parentElement;
      if (!parent) {
        parts.unshift(tag);
        break;
      }
      const siblings = Array.from(parent.children).filter((child) => child.tagName === node.tagName);
      parts.unshift(siblings.length > 1 ? `${tag}:nth-of-type(${siblings.indexOf(node) + 1})` : tag);
      node = parent;
    }
    return parts.join(' > ');
  };
  const nameFor = (el) => {
    const values = [el.getAttribute('aria-label'), el.getAttribute('placeholder'), el.getAttribute('title'), (el.innerText || el.textContent || '').trim().slice(0, 120), el.getAttribute('name')];
    return values.map((value) => String(value || '').replace(/\s+/g, ' ').trim()).find(Boolean) || '';
  };
  const elements = [];
  for (const el of document.querySelectorAll(query)) {
    if (elements.length >= limit) break;
    if (!visible(el)) continue;
    const tag = el.tagName.toLowerCase();
    const item = { role: el.getAttribute('role') || (tag === 'a' ? 'link' : tag === 'button' ? 'button' : tag), tag, name: nameFor(el), selector: selectorFor(el) };
    if (['input', 'textarea', 'select'].includes(tag)) {
      item.type = el.getAttribute('type') || tag;
      item.value = typeof el.value === 'string' ? el.value.slice(0, 200) : '';
      if (el.disabled) item.disabled = true;
      if (typeof el.checked === 'boolean' && ['checkbox', 'radio'].includes(item.type)) item.checked = el.checked;
    }
    if (tag === 'a' && el.href) item.href = String(el.href).slice(0, 500);
    elements.push(item);
  }
  return { url: location.href, title: document.title, elements, truncated: elements.length >= limit };
}

async function run(payload) {
  const runtimeDir = String(payload.runtimeDir || '').trim();
  if (!runtimeDir) throw new Error('runtimeDir is required');
  const { chromium } = require(path.join(runtimeDir, 'node_modules', 'playwright-core'));
  const defaultTimeout = normalizeTimeout(payload.defaultTimeoutMs, DEFAULT_TIMEOUT_MS);
  const artifactDir = String(payload.artifactDir || '').trim();
  const gateway = createBrowserGateway(payload, chromium, { defaultTimeout });
  const launchSession = await gateway.launch({});
  const connection = await gateway.connect(launchSession, { timeoutMs: payload.connectTimeoutMs });
  const { browser, context } = await gateway.resolveConnectionContext(connection);
  let active = connection.page || null;

  const pages = () => context.pages().filter((page) => !page.isClosed());
  const activePage = async () => {
    if (active && !active.isClosed()) return active;
    active = pages()[0] || (await context.newPage());
    return active;
  };
  const describePage = async (page) => {
    let title = '';
    try { title = await page.title(); } catch {}
    return { url: page && !page.isClosed() ? page.url() : '', title };
  };
  const locatorFor = async (args = {}) => {
    const page = await activePage();
    const selector = String(args.selector || '').trim();
    if (!selector) throw new Error('selector is required');
    const scope = args.frameSelector ? page.frameLocator(String(args.frameSelector)) : page;
    const locator = scope.locator(selector);
    const index = Number(args.index);
    return Number.isFinite(index) && index >= 0 ? locator.nth(index) : locator.first();
  };
  const timeoutOf = (args) => normalizeTimeout(args && args.timeoutMs, defaultTimeout);

  const execute = async (action, args = {}) => {
    const page = await activePage();
    switch (action) {
      case 'goto': {
        const url = String(args.url || '').trim();
        if (!url) throw new Error('url is required');
        const response = await page.goto(url, { waitUntil: args.waitUntil || 'domcontentloaded', timeout: timeoutOf(args) });
        return { ...(await describePage(page)), status: response ? response.status() : 0 };
      }
      case 'snapshot':
        return await page.evaluate(collectInteractiveElements, { limit: Number(args.limit) || DEFAULT_SNAPSHOT_LIMIT });
      case 'click': {
        const locator = await locatorFor(args);
        await locator.click({ timeout: timeoutOf(args), button: args.button || 'left', clickCount: Number(args.clickCount) || 1, force: Boolean(args.force) });
        return await describePage(page);
      }
      case 'fill': {
        const fields = Array.isArray(args.fields) ? args.fields : [{ selector: args.selector, value: args.value, frameSelector: args.frameSelector }];
        const filled = [];
        for (const field of fields) {
          const locator = await locatorFor(field || {});
          await locator.fill(String((field && field.value) ?? ''), { timeout: timeoutOf(args) });
          filled.push(String(field.selector || ''));
        }
        return { ...(await describePage(page)), filled };
      }
      case 'press': {
        const inputToken = String(args.press || '').trim();
        if (!inputToken) throw new Error('press input is required');
        if (args.selector) {
          const locator = await locatorFor(args);
          await locator.press(inputToken, { timeout: timeoutOf(args) });
        } else {
          await page.keyboard.press(key);
        }
        return await describePage(page);
      }
      case 'selectOption': {
        const locator = await locatorFor(args);
        const values = Array.isArray(args.values) ? args.values.map(String) : [String(args.value || '')].filter(Boolean);
        if (values.length === 0) throw new Error('values are required');
        const selected = await locator.selectOption(values, { timeout: timeoutOf(args) });
        return { ...(await describePage(page)), selected };
      }
      case 'waitFor': {
        const timeout = timeoutOf(args);
        if (args.selector) {
          const locator = await locatorFor(args);
          await locator.waitFor({ state: args.state || 'visible', timeout });
        } else if (args.url) {
          await page.waitForURL(String(args.url), { timeout, waitUntil: args.waitUntil || 'domcontentloaded' });
        } else if (args.loadState) {
          await page.waitForLoadState(String(args.loadState), { timeout });
        } else {
          await page.waitForTimeout(timeout);
        }
        return await describePage(page);
      }
      case 'extract': {
        const locator = await locatorFor(args);
        const mode = String(args.mode || 'text').toLowerCase();
        const all = Boolean(args.all);
        const target = all ? locator : locator.first();
        let values;
        if (mode === 'html') values = all ? await target.evaluateAll((els) => els.map((el) => el.outerHTML)) : await target.evaluate((el) => el.outerHTML);
        else if (mode === 'attribute') {
          const name = String(args.attribute || '').trim();
          if (!name) throw new Error('attribute is required');
          values = all ? await target.evaluateAll((els, attr) => els.map((el) => el.getAttribute(attr)), name) : await target.getAttribute(name);
        } else if (mode === 'value') values = all ? await target.evaluateAll((els) => els.map((el) => el.value ?? '')) : await target.inputValue();
        else values = all ? await target.allTextContents() : await target.textContent();
        return { mode, value: all ? undefined : String(values ?? ''), values: all ? toSerializable(values) : undefined };
      }
      case 'evaluate': {
        const expression = String(args.expression || '').trim();
        if (!expression) throw new Error('expression is required');
        const arg = args.arg;
        const value = await page.evaluate(({ source, input }) => {
          const candidate = (0, eval)(source);
          return typeof candidate === 'function' ? candidate(input) : candidate;
        }, { source: expression, input: arg });
        return { value: toSerializable(value) };
      }
      case 'screenshot': {
        if (!artifactDir) throw new Error('artifactDir is required for screenshots');
        fs.mkdirSync(artifactDir, { recursive: true });
        const format = String(args.format || 'jpeg').toLowerCase() === 'png' ? 'png' : 'jpeg';
        const filename = String(args.filename || `page-${Date.now()}.${format === 'png' ? 'png' : 'jpg'}`);
        const outputPath = normalizePathUnderRoot(artifactDir, filename);
        const options = { path: outputPath, type: format, fullPage: Boolean(args.fullPage) };
        if (format === 'jpeg') options.quality = Math.max(10, Math.min(100, Number(args.quality) || 70));
        if (args.selector) {
          const locator = await locatorFor(args);
          await locator.screenshot(options);
        } else {
          await page.screenshot(options);
        }
        return { ...(await describePage(page)), path: outputPath, mimeType: format === 'png' ? 'image/png' : 'image/jpeg' };
      }
      case 'tabs': {
        const op = String(args.op || 'list').toLowerCase();
        if (op === 'new') {
          active = await context.newPage();
          if (args.url) await active.goto(String(args.url), { waitUntil: 'domcontentloaded', timeout: timeoutOf(args) });
        } else if (op === 'select') {
          const index = Number(args.index);
          const list = pages();
          if (!Number.isInteger(index) || index < 0 || index >= list.length) throw new Error('tab index is out of range');
          active = list[index];
          await active.bringToFront();
        } else if (op === 'close') {
          const list = pages();
          const index = args.index == null ? list.indexOf(active) : Number(args.index);
          if (!Number.isInteger(index) || index < 0 || index >= list.length) throw new Error('tab index is out of range');
          await list[index].close();
          active = pages()[0] || null;
        } else if (op !== 'list') {
          throw new Error('unsupported tabs op');
        }
        const list = pages();
        const items = [];
        for (let index = 0; index < list.length; index += 1) {
          items.push({ index, active: list[index] === active, ...(await describePage(list[index])) });
        }
        return { count: items.length, items, page: active ? await describePage(active) : { url: '', title: '' } };
      }
      default:
        throw new Error(`unsupported page action: ${action}`);
    }
  };

  let writeChain = Promise.resolve();
  const emit = (message) => {
    writeChain = writeChain.then(() => new Promise((resolve) => process.stdout.write(`${JSON.stringify(message)}\n`, resolve)));
    return writeChain;
  };
  await emit({ type: 'ready', session: { ...launchSession, cdpUrl: connection.session && connection.session.cdpUrl } });

  const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
  rl.on('line', async (line) => {
    const trimmed = String(line || '').trim();
    if (!trimmed) return;
    let command;
    try { command = JSON.parse(trimmed); } catch (error) { await emit({ id: 0, ok: false, error: `invalid command: ${error.message}` }); return; }
    try {
      const result = await execute(String(command.action || '').trim(), command.args || {});
      await emit({ id: Number(command.id) || 0, ok: true, result: toSerializable(result) });
    } catch (error) {
      await emit({ id: Number(command.id) || 0, ok: false, error: error && error.message ? error.message : String(error) });
    }
  });

  const shutdown = async (reason) => {
    try { await emit({ type: 'closed', reason }); } catch {}
    try { await gateway.closeAll(); } catch {}
    process.exit(0);
  };
  rl.on('close', () => void shutdown('stdin closed'));
  browser.on('disconnected', () => void shutdown('browser disconnected'));
  process.on('SIGINT', () => void shutdown('SIGINT'));
  process.on('SIGTERM', () => void shutdown('SIGTERM'));
}

async function main() {
  const payloadPath = process.argv[2];
  if (!payloadPath) throw new Error('payload path is required');
  const payload = JSON.parse(fs.readFileSync(payloadPath, 'utf8'));
  await run(payload);
}

main().catch((error) => {
  process.stdout.write(`${JSON.stringify({ type: 'closed', reason: error && error.message ? error.message : String(error) })}\n`);
  process.exitCode = 1;
});
