const FALLBACK_BASE_URL = process.env.PLAYWRIGHT_FALLBACK_BASE_URL || 'http://backend:8080';

function isUsableHostname(hostname) {
  return Boolean(hostname) && !hostname.includes('_');
}

function resolveBaseUrl() {
  const raw = process.env.BASE_URL;
  if (!raw) {
    return FALLBACK_BASE_URL;
  }

  try {
    const url = new URL(raw);
    if (!isUsableHostname(url.hostname)) {
      console.warn(`Playwright overriding BASE_URL='${raw}' due to unsupported hostname`);
      return FALLBACK_BASE_URL;
    }
    return raw;
  } catch (err) {
    console.warn(`Playwright overriding BASE_URL='${raw}' due to parse error: ${err}`);
    return FALLBACK_BASE_URL;
  }
}

export const BASE_URL = resolveBaseUrl();
export const BASE_HOST = new URL(BASE_URL).hostname;
