"use strict";
const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const { createManager, COOKIE_SECONDS } = require("./remembered-session.cjs");

async function fixture(t) {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), "mg-remembered-test-"));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  let time = Date.parse("2026-10-11T00:00:00Z"), rotations = 0, offline = false, active = true, logouts = 0;
  const fetch = async (url, options) => {
    if (offline) throw new TypeError("offline");
    if (url.endsWith("/auth/introspect")) return Response.json({ active });
    const body = JSON.parse(options.body);
    if (body.query.includes("RememberedDeviceLogout")) { logouts++; active = false; return Response.json({ data: { logout: true } }); }
    if (!active) return Response.json({ errors: [{ message: "invalid refresh token" }] });
    rotations++;
    return Response.json({ data: { refreshToken: { accessToken: "access-" + rotations, refreshToken: "atom_rt_secret-" + rotations, accessTokenExpiresAt: new Date(time + 3600000).toISOString(), refreshTokenExpiresAt: new Date(time + 3153600000000).toISOString(), sessionId: "session-1" } } });
  };
  const options = { directory, secret: "test-encryption-secret", baseUrl: "http://atom", fetch, now: () => time };
  const manager = createManager(options);
  const cookie = await manager.jwt({ token: { sub: "user-1" }, trigger: "signIn", account: {}, user: { user: { id: "user-1" }, workspace: { id: "workspace-1" }, accessToken: "access-0", refreshToken: "atom_rt_secret-0", accessTokenExpiry: time + 3600000, refreshTokenExpiry: time + 3153600000000, sessionId: "session-1" } });
  return { directory, manager, cookie, restart: () => createManager(options), advance: (value) => { time += value; }, offline: (value) => { offline = value; }, revoke: () => { active = false; }, rotations: () => rotations, logouts: () => logouts };
}

test("expired access token restores, concurrent stale cookies rotate exactly once", async (t) => {
  const f = await fixture(t);
  f.advance(7200000);
  const results = await Promise.all(Array.from({ length: 30 }, () => f.manager.jwt({ token: f.cookie })));
  assert.equal(f.rotations(), 1);
  for (const token of results) { assert.equal(token.accessToken, "access-1"); assert.equal(token.rememberId, f.cookie.rememberId); assert.equal(token.error, undefined); }
});
test("encrypted persisted replacement survives restart and contains no plaintext credentials", async (t) => {
  const f = await fixture(t); f.advance(7200000);
  await f.manager.jwt({ token: f.cookie });
  const token = await f.restart().jwt({ token: f.cookie });
  assert.equal(token.accessToken, "access-1"); assert.equal(f.rotations(), 1);
  const raw = await fs.readFile(path.join(f.directory, f.cookie.rememberId + ".json"), "utf8");
  assert.ok(!raw.includes("access-1")); assert.ok(!raw.includes("atom_rt_secret"));
  assert.equal(token.refreshToken, undefined);
  assert.equal(f.manager.session({ session: {}, token }).rememberId, token.rememberId);
  assert.equal(f.manager.session({ session: {}, token }).refreshToken, undefined);
});
test("network loss preserves login, and reconnect restores expired access", async (t) => {
  const f = await fixture(t); f.advance(7200000); f.offline(true);
  const waiting = await f.manager.jwt({ token: f.cookie });
  assert.equal(waiting.error, undefined); assert.equal(waiting.sessionUnavailable, true);
  f.offline(false);
  const restored = await f.manager.jwt({ token: waiting });
  assert.equal(restored.accessToken, "access-1"); assert.equal(restored.sessionUnavailable, undefined);
});
test("revoked session requires login instead of being restored", async (t) => {
  const f = await fixture(t); f.advance(61000); f.revoke();
  const result = await f.manager.jwt({ token: f.cookie });
  assert.equal(result.error, "SessionRevoked"); assert.equal(result.accessToken, "");
});
test("explicit logout revokes Atom and prevents replay after restart", async (t) => {
  const f = await fixture(t);
  await f.manager.signOut({ token: f.cookie });
  assert.equal(f.logouts(), 1);
  const result = await f.restart().jwt({ token: f.cookie });
  assert.equal(result.error, "SessionRevoked");
});
test("offline logout remains revoked and completes when Atom reconnects", async (t) => {
  const f = await fixture(t); f.advance(7200000); f.offline(true);
  await f.manager.signOut({ token: f.cookie });
  assert.equal((await f.restart().jwt({ token: f.cookie })).error, "SessionRevoked");
  f.offline(false); await f.manager.retryPendingLogouts();
  assert.equal(f.logouts(), 1); assert.equal(f.rotations(), 1);
});
test("workspace updates retain device identity and reject injected credentials", async (t) => {
  const f = await fixture(t);
  const result = await f.manager.jwt({ token: f.cookie, trigger: "update", session: { workspace: { id: "workspace-2" }, accessToken: "injected", rememberId: "injected", user: { id: "other" } } });
  assert.equal(result.workspace.id, "workspace-2"); assert.equal(result.rememberId, f.cookie.rememberId); assert.equal(result.accessToken, "access-0"); assert.equal(result.user.id, "user-1");
});
test("cleared or malformed device identity cannot restore a session", async (t) => {
  const f = await fixture(t);
  assert.equal((await f.manager.jwt({ token: {} })).error, "SessionRevoked");
  assert.equal((await f.manager.jwt({ token: { ...f.cookie, rememberId: "../anything" } })).error, "SessionRevoked");
  assert.equal(COOKIE_SECONDS, 34560000);
});
test("workspace cookie re-encoding persists metadata so old cookie responses cannot undo a selection", async (t) => {
  const f = await fixture(t);
  const current = f.manager.session({ session: {}, token: f.cookie });
  const encoded = await f.manager.forEncoding({ ...current, workspace: { id: "workspace-2" } });
  assert.equal(encoded.workspace.id, "workspace-2");
  assert.equal(encoded.rememberId, f.cookie.rememberId);
  const stale = await f.restart().jwt({ token: f.cookie });
  assert.equal(stale.workspace.id, "workspace-2");
  assert.equal(stale.user.id, "user-1");
});
