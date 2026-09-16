const {
  normalizeTimeout,
  sleep,
  closeBrowserConnection,
  buildConnectEndpoints,
  requestJSON,
} = require('./runner_shared.cjs');

const LAUNCH_BODY_KEYS = [
  'code',
  'key',
  'profileId',
  'profileName',
  'keyword',
  'keywords',
  'tag',
  'tags',
  'groupId',
  'matchMode',
  'proxyId',
  'proxyConfig',
  'launchArgs',
  'startUrls',
  'skipDefaultStartUrls',
];

function buildLaunchRequestBody(defaultSelector, options) {
  const launchOptions = options && typeof options === 'object' ? options : {};
  const body = {};
  for (const fieldName of LAUNCH_BODY_KEYS) {
    if (Object.prototype.hasOwnProperty.call(launchOptions, fieldName)) {
      body[fieldName] = launchOptions[fieldName];
    }
  }
  const selector =
    launchOptions.selector && typeof launchOptions.selector === 'object' && !Array.isArray(launchOptions.selector)
      ? launchOptions.selector
      : defaultSelector;
  if (selector && typeof selector === 'object' && !Array.isArray(selector) && Object.keys(selector).length > 0) {
    body.selector = selector;
  }
  if (!Object.prototype.hasOwnProperty.call(body, 'skipDefaultStartUrls')) {
    body.skipDefaultStartUrls = true;
  }
  return body;
}

function createBrowserGateway(payload, chromium, options = {}) {
  const defaultTimeout = normalizeTimeout(options.defaultTimeout, 30000);
  const selector = payload && payload.selector && typeof payload.selector === 'object' ? payload.selector : {};
  const connectedBrowsers = new Set();
  const launchHeaders = {};
  if (payload && payload.launchAuthHeader && payload.launchAuthValue) {
    launchHeaders[payload.launchAuthHeader] = payload.launchAuthValue;
  }

  const launch = async (launchOptions = {}) => {
    const response = await requestJSON(
      'POST',
      `${String((payload && payload.launchBaseUrl) || '').replace(/\/$/, '')}/api/launch`,
      buildLaunchRequestBody(selector, launchOptions),
      launchHeaders
    );
    if (!(response.status >= 200 && response.status < 300) || response.body.ok === false) {
      const message =
        (response.body && response.body.error && String(response.body.error).trim()) ||
        `launch api returned http ${response.status}`;
      throw new Error(message);
    }
    return response.body;
  };

  const connect = async (session = {}, connectOptions = {}) => {
    const endpoints = buildConnectEndpoints(payload, session);
    if (endpoints.length === 0) {
      throw new Error('launch session does not contain a valid cdp endpoint');
    }
    const connectTimeout = normalizeTimeout(connectOptions.timeoutMs, defaultTimeout);
    const deadline = Date.now() + connectTimeout;
    let lastError = null;
    while (Date.now() <= deadline) {
      for (const endpoint of endpoints) {
        const remaining = deadline - Date.now();
        if (remaining <= 0) break;
        try {
          const browser = await chromium.connectOverCDP(endpoint, {
            timeout: Math.max(1000, Math.min(remaining, connectTimeout)),
          });
          connectedBrowsers.add(browser);
          const context = browser.contexts()[0] || null;
          const page = context && context.pages().length > 0 ? context.pages()[0] : null;
          return { browser, context, page, session: { ...session, cdpUrl: endpoint } };
        } catch (error) {
          lastError = error;
        }
      }
      if (Date.now() >= deadline) break;
      await sleep(Math.min(500, Math.max(100, deadline - Date.now())));
    }
    const lastMessage = lastError && lastError.message ? lastError.message : String(lastError || 'unknown error');
    throw new Error(`cdp endpoint is not ready after ${connectTimeout} ms: ${lastMessage}`);
  };

  const resolveConnectionContext = async (connection) => {
    const browser = connection && connection.browser ? connection.browser : null;
    if (!browser) throw new Error('browser connection is unavailable');
    const context =
      connection.context ||
      browser.contexts()[0] ||
      (typeof browser.newContext === 'function' ? await browser.newContext() : null);
    if (!context) throw new Error('browser context is unavailable');
    return { browser, context };
  };

  const closeAll = async () => {
    await Promise.all(Array.from(connectedBrowsers, (browser) => closeBrowserConnection(browser)));
    connectedBrowsers.clear();
  };

  return { selector, launch, connect, resolveConnectionContext, connectedBrowsers, closeAll };
}

module.exports = { buildLaunchRequestBody, createBrowserGateway };
