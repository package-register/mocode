const AUTH_TOKEN_KEY = "mocode_auth_token";
const LEGACY_AUTH_TOKEN_KEY = "kimi_auth_token";
const AUTH_TOKEN_TIMESTAMP_KEY = "mocode_auth_token_ts";
const LEGACY_AUTH_TOKEN_TIMESTAMP_KEY = "kimi_auth_token_ts";
const AUTH_TOKEN_PARAM = "token";
const TOKEN_EXPIRY_MS = 24 * 60 * 60 * 1000; // 24 hours

/** Read a localStorage value, falling back to its legacy key. */
function getItemWithLegacy(key: string, legacyKey: string): string | null {
  const val = localStorage.getItem(key);
  if (val !== null) return val;
  const legacyVal = localStorage.getItem(legacyKey);
  if (legacyVal !== null) {
    // Migrate on read.
    localStorage.setItem(key, legacyVal);
    localStorage.removeItem(legacyKey);
  }
  return legacyVal;
}

/** Set a localStorage value and clear the legacy counterpart. */
function setItemWithLegacy(key: string, legacyKey: string, value: string): void {
  localStorage.setItem(key, value);
  localStorage.removeItem(legacyKey);
}

export function getAuthToken(): string | null {
  const token = getItemWithLegacy(AUTH_TOKEN_KEY, LEGACY_AUTH_TOKEN_KEY);
  if (!token) {
    return null;
  }

  // Check if token has expired.
  const timestamp = getItemWithLegacy(AUTH_TOKEN_TIMESTAMP_KEY, LEGACY_AUTH_TOKEN_TIMESTAMP_KEY);
  if (timestamp) {
    const storedAt = parseInt(timestamp, 10);
    if (Number.isNaN(storedAt)) {
      clearAuthToken();
      return null;
    }
    const age = Date.now() - storedAt;
    if (age > TOKEN_EXPIRY_MS) {
      clearAuthToken();
      return null;
    }
  }

  return token;
}

export function setAuthToken(token: string): void {
  setItemWithLegacy(AUTH_TOKEN_KEY, LEGACY_AUTH_TOKEN_KEY, token);
  setItemWithLegacy(AUTH_TOKEN_TIMESTAMP_KEY, LEGACY_AUTH_TOKEN_TIMESTAMP_KEY, Date.now().toString());
}

export function clearAuthToken(): void {
  localStorage.removeItem(AUTH_TOKEN_KEY);
  localStorage.removeItem(LEGACY_AUTH_TOKEN_KEY);
  localStorage.removeItem(AUTH_TOKEN_TIMESTAMP_KEY);
  localStorage.removeItem(LEGACY_AUTH_TOKEN_TIMESTAMP_KEY);
}

export function consumeAuthTokenFromUrl(): string | null {
  const url = new URL(window.location.href);
  const token = url.searchParams.get(AUTH_TOKEN_PARAM);
  if (!token) {
    return null;
  }
  url.searchParams.delete(AUTH_TOKEN_PARAM);
  window.history.replaceState({}, "", url.toString());
  return token;
}

export function getAuthHeader(): Record<string, string> {
  let token = getAuthToken();
  if (!token) {
    const url = new URL(window.location.href);
    token = url.searchParams.get(AUTH_TOKEN_PARAM);
  }
  if (!token) {
    return {};
  }
  return { Authorization: `Bearer ${token}` };
}
