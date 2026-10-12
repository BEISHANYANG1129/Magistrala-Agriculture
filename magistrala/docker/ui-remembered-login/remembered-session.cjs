/* Remembered devices for the pinned Magistrala community UI. No browser passwords or refresh tokens. */
"use strict";
const fs = require("node:fs/promises");
const path = require("node:path");
const crypto = require("node:crypto");

const COOKIE_SECONDS = 400 * 86400;
const RENEW_BEFORE_MS = 5 * 60000;
const CHECK_EVERY_MS = 60000;
class Revoked extends Error {}

function createManager({ directory, secret, baseUrl, fetch: fetcher = globalThis.fetch, now = Date.now }) {
  if (!secret || !baseUrl) throw new Error("Remembered login requires NEXTAUTH_SECRET and MG_ATOM_URL");
  const key = crypto.createHash("sha256").update("magistrala-remembered-device-v1\0" + secret).digest();
  const locks = new Map();
  const filename = (id) => {
    if (!/^[a-f0-9]{64}$/.test(id || "")) throw new Revoked("Missing remembered device");
    return path.join(directory, id + ".json");
  };
  async function locked(id, action) {
    const previous = locks.get(id) || Promise.resolve();
    const current = previous.catch(() => {}).then(action);
    locks.set(id, current);
    try { return await current; } finally { if (locks.get(id) === current) locks.delete(id); }
  }
  async function save(id, state) {
    await fs.mkdir(directory, { recursive: true, mode: 0o700 });
    const iv = crypto.randomBytes(12);
    const cipher = crypto.createCipheriv("aes-256-gcm", key, iv);
    cipher.setAAD(Buffer.from(id));
    const data = Buffer.concat([cipher.update(JSON.stringify(state)), cipher.final()]);
    const value = JSON.stringify({ v: 1, iv: iv.toString("base64"), tag: cipher.getAuthTag().toString("base64"), data: data.toString("base64") });
    const temporary = filename(id) + "." + crypto.randomBytes(8).toString("hex") + ".tmp";
    await fs.writeFile(temporary, value, { mode: 0o600, flag: "wx" });
    await fs.rename(temporary, filename(id));
  }
  async function load(id) {
    let raw;
    try { raw = await fs.readFile(filename(id), "utf8"); }
    catch (error) { if (error.code === "ENOENT") throw new Revoked("Remembered device was removed"); throw error; }
    try {
      const value = JSON.parse(raw);
      const decipher = crypto.createDecipheriv("aes-256-gcm", key, Buffer.from(value.iv, "base64"));
      decipher.setAAD(Buffer.from(id));
      decipher.setAuthTag(Buffer.from(value.tag, "base64"));
      return JSON.parse(Buffer.concat([decipher.update(Buffer.from(value.data, "base64")), decipher.final()]).toString());
    } catch { throw new Revoked("Remembered device credentials cannot be decrypted"); }
  }
  async function json(endpoint, options = {}) {
    const response = await fetcher(baseUrl.replace(/\/+$/, "") + endpoint, {
      ...options, cache: "no-store", signal: AbortSignal.timeout(10000),
      headers: { Accept: "application/json", "Content-Type": "application/json", ...options.headers },
    });
    const data = await response.json();
    const message = (data.errors || []).map((error) => error.message || "").join("; ") || data.error || data.message || "";
    if (response.status === 401 || /invalid refresh token|session revoked|session is not refreshable|entity is inactive|tenant is inactive/i.test(String(message))) {
      throw new Revoked("Atom revoked the remembered session");
    }
    if (!response.ok || data.errors?.length) throw new Error("Atom session service is temporarily unavailable");
    return data;
  }
  function pair(state, response) {
    const accessTokenExpiry = Date.parse(response.accessTokenExpiresAt);
    const refreshTokenExpiry = Date.parse(response.refreshTokenExpiresAt);
    if (!response.accessToken || !response.refreshToken?.startsWith("atom_rt_") || !Number.isFinite(accessTokenExpiry) || !Number.isFinite(refreshTokenExpiry)) {
      throw new Error("Atom did not issue a valid remembered session");
    }
    return { ...state, accessToken: response.accessToken, refreshToken: response.refreshToken, accessTokenExpiry, refreshTokenExpiry, sessionId: response.sessionId || state.sessionId, checkedAt: now() };
  }
  async function renew(state) {
    const result = await json("/graphql", {
      method: "POST", body: JSON.stringify({ query: "mutation RememberedDeviceRefresh($input:RefreshTokenInput!){refreshToken(input:$input){accessToken accessTokenExpiresAt refreshToken refreshTokenExpiresAt sessionId}}", variables: { input: { refreshToken: state.refreshToken } } }),
    });
    return pair(state, result.data?.refreshToken || {});
  }
  async function check(state) {
    const result = await json("/auth/introspect", { headers: { Authorization: "Bearer " + state.accessToken } });
    if (result.active === false) throw new Revoked("Session revoked");
    if (result.active !== true) throw new Error("Invalid session check response");
    return { ...state, checkedAt: now() };
  }
  function cookie(token, id, state) {
    const { refreshToken: _refresh, error: _error, sessionUnavailable: _unavailable, ...safe } = token;
    return { ...safe, rememberId: id, user: state.user || token.user, workspace: state.workspace === undefined ? token.workspace : state.workspace || undefined, sessionId: state.sessionId, accessToken: state.accessToken, accessTokenExpiry: state.accessTokenExpiry, refreshTokenExpiry: state.refreshTokenExpiry };
  }
  async function start(token, user) {
    const state = {
      accessToken: user.accessToken, accessTokenExpiry: Number(user.accessTokenExpiry),
      refreshToken: user.refreshToken, refreshTokenExpiry: Number(user.refreshTokenExpiry),
      sessionId: user.sessionId, checkedAt: now(), createdAt: now(),
      user: user.user, workspace: user.workspace || null,
    };
    if (!state.refreshToken?.startsWith("atom_rt_") || !state.sessionId || !Number.isFinite(state.refreshTokenExpiry)) throw new Error("Atom refresh tokens are not enabled");
    const id = crypto.randomBytes(32).toString("hex");
    await save(id, state);
    return cookie({ ...token, user: user.user, workspace: user.workspace }, id, state);
  }
  async function resolve(token, session, trigger) {
    return locked(token.rememberId, async () => {
      try {
        let state = await load(token.rememberId);
        if (state.revoked || state.refreshTokenExpiry <= now()) throw new Revoked("Remembered session revoked or expired");
        if (state.accessTokenExpiry - now() <= RENEW_BEFORE_MS) {
          state = await renew(state);
          // Persist before responding: stale cookies and concurrent RSC/API calls reuse this replacement.
          await save(token.rememberId, state);
        } else if (now() - state.checkedAt >= CHECK_EVERY_MS) {
          state = await check(state);
          await save(token.rememberId, state);
        }
        // Preserve the UI's workspace selector without accepting browser-supplied auth credentials.
        if (trigger === "update" && session?.workspace) {
          state = { ...state, workspace: session.workspace };
          await save(token.rememberId, state);
        }
        return cookie(token, token.rememberId, state);
      } catch (error) {
        if (error instanceof Revoked) return { ...token, accessToken: "", error: "SessionRevoked" };
        // A timeout/offline/5xx is not a logout. Keep the device cookie and retry on focus/online.
        const { error: _error, ...safe } = token;
        return { ...safe, sessionUnavailable: true };
      }
    });
  }
  async function finishLogout(id, state) {
    if (state.accessTokenExpiry <= now()) {
      state = await renew(state);
      await save(id, { ...state, revoked: true, pendingLogout: true });
    }
    await json("/graphql", { method: "POST", headers: { Authorization: "Bearer " + state.accessToken }, body: JSON.stringify({ query: "mutation RememberedDeviceLogout{logout}" }) });
    await save(id, { revoked: true, sessionId: state.sessionId, revokedAt: now() });
  }
  async function signOut({ token } = {}) {
    if (!token?.rememberId) return;
    await locked(token.rememberId, async () => {
      let state;
      try { state = await load(token.rememberId); } catch { return; }
      if (state.revoked && !state.pendingLogout) return;
      state = { ...state, revoked: true, pendingLogout: true };
      // Local revocation is durable even if the backend is offline during explicit logout.
      await save(token.rememberId, state);
      try { await finishLogout(token.rememberId, state); }
      catch (error) { if (error instanceof Revoked) await save(token.rememberId, { revoked: true, sessionId: state.sessionId, revokedAt: now() }); }
    });
  }
  async function retryPendingLogouts() {
    const entries = await fs.readdir(directory).catch(() => []);
    for (const name of entries) {
      if (!/^[a-f0-9]{64}\.json$/.test(name)) continue;
      const id = name.slice(0, -5);
      await locked(id, async () => {
        try {
          const state = await load(id);
          if (state.pendingLogout) await finishLogout(id, state);
        } catch (error) {
          if (error instanceof Revoked) await save(id, { revoked: true, revokedAt: now() });
        }
      });
    }
  }
  return {
    async forEncoding(token) {
      if (!token.rememberId) return token;
      return locked(token.rememberId, async () => {
        const state = await load(token.rememberId);
        if (state.revoked) return { ...token, error: "SessionRevoked", accessToken: "" };
        const updated = { ...state, workspace: token.workspace || null };
        await save(token.rememberId, updated);
        return cookie(token, token.rememberId, updated);
      });
    },
    async jwt({ token, user, account, session, trigger }) {
      if (trigger === "signIn") return user && account ? start(token, user) : { ...token, error: "SessionRevoked" };
      if (!token.rememberId) return { ...token, error: "SessionRevoked" };
      return resolve(token, session, trigger);
    },
    session({ session, token }) {
      // The upstream workspace selector re-encodes the public session into its signed cookie.
      // Keep the opaque locator through that round trip; it grants no access without the signed cookie.
      return { ...session, rememberId: token.rememberId, user: token.user, workspace: token.workspace, accessToken: token.accessToken, accessTokenExpiry: token.accessTokenExpiry, refreshTokenExpiry: token.refreshTokenExpiry, error: token.error, sessionUnavailable: token.sessionUnavailable };
    },
    signOut, retryPendingLogouts,
  };
}

function instance() {
  const symbol = Symbol.for("magistrala.remembered-device.manager.v1");
  if (!globalThis[symbol]) {
    const manager = createManager({ directory: process.env.MG_REMEMBERED_SESSION_DIR || "/var/lib/magistrala-ui/sessions", secret: process.env.NEXTAUTH_SECRET, baseUrl: process.env.MG_ATOM_URL || process.env.ATOM_URL });
    const timer = setInterval(() => manager.retryPendingLogouts().catch(() => {}), 60000);
    timer.unref();
    globalThis[symbol] = manager;
    manager.retryPendingLogouts().catch(() => {});
  }
  return globalThis[symbol];
}
module.exports = { COOKIE_SECONDS, createManager, jwt: (args) => instance().jwt(args), session: (args) => instance().session(args), signOut: (args) => instance().signOut(args), forEncoding: (args) => instance().forEncoding(args) };
