"use strict";
// Uses the configured seeded admin credential internally; never prints credentials or cookies.
const assert = require("node:assert/strict");
const { execFileSync } = require("node:child_process");
const inspect = JSON.parse(execFileSync("docker", ["inspect", "magistrala-atom"], { encoding: "utf8" }))[0];
const env = Object.fromEntries(inspect.Config.Env.map((line) => { const i = line.indexOf("="); return [line.slice(0, i), line.slice(i + 1)]; }));
const secret = env.ADMIN_SECRET;
if (!secret) throw new Error("Seeded admin credential unavailable for live verification");
const base = "http://localhost:3000";
const testJars = new Set();
class Jar {
  constructor() { this.cookies = new Map(); this.expires = new Map(); }
  header() { return [...this.cookies].map(([name, value]) => name + "=" + value).join("; "); }
  capture(response) {
    for (const raw of response.headers.getSetCookie()) {
      const parts = raw.split(";"), at = parts[0].indexOf("="), name = parts[0].slice(0, at), value = parts[0].slice(at + 1);
      if (parts.some((part) => /max-age=0/i.test(part)) || !value) this.cookies.delete(name);
      else this.cookies.set(name, value);
      const expires = parts.find((part) => /expires=/i.test(part));
      if (expires) this.expires.set(name, Date.parse(expires.slice(expires.indexOf("=") + 1).trim()));
    }
  }
}
async function request(jar, route, options = {}) {
  const response = await fetch(base + route, { ...options, redirect: "manual", headers: { ...options.headers, Cookie: jar.header() }, signal: AbortSignal.timeout(15000) });
  jar.capture(response);
  return response;
}
async function signIn() {
  const jar = new Jar();
  testJars.add(jar);
  const csrf = await (await request(jar, "/api/auth/csrf")).json();
  const body = new URLSearchParams({ csrfToken: csrf.csrfToken, identity: "admin", password: secret, callbackUrl: base + "/en", json: "true" });
  const response = await request(jar, "/api/auth/callback/credentials", { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body });
  const result = await response.json();
  assert.ok(!result.url?.includes("error="), "Configured admin login failed");
  const session = await (await request(jar, "/api/auth/session")).json();
  assert.equal(session.user?.id, "00000000-0000-0000-0000-000000000001");
  assert.equal(session.error, undefined);
  assert.equal(session.refreshToken, undefined);
  const name = [...jar.cookies.keys()].find((key) => key.includes("session-token"));
  assert.ok(jar.expires.get(name) - Date.now() > 399 * 86400000, "Cookie must remember the browser after close");
  return { jar, session };
}
const helper = `
const fs=require("fs"),crypto=require("crypto"),path=require("path");
(async()=>{let input="";for await(const chunk of process.stdin)input+=chunk;const payload=JSON.parse(input);
const pieces=payload.cookie.split("."),jwtKey=crypto.hkdfSync("sha256",process.env.NEXTAUTH_SECRET,"","NextAuth.js Generated Encryption Key",32),jwtDecipher=crypto.createDecipheriv("aes-256-gcm",jwtKey,Buffer.from(pieces[2],"base64url"));jwtDecipher.setAAD(Buffer.from(pieces[0]));jwtDecipher.setAuthTag(Buffer.from(pieces[4],"base64url"));
const token=JSON.parse(Buffer.concat([jwtDecipher.update(Buffer.from(pieces[3],"base64url")),jwtDecipher.final()]).toString());
const id=token.rememberId,file=path.join(process.env.MG_REMEMBERED_SESSION_DIR,id+".json");
const key=crypto.createHash("sha256").update("magistrala-remembered-device-v1\\0"+process.env.NEXTAUTH_SECRET).digest();
const raw=JSON.parse(fs.readFileSync(file,"utf8")),decipher=crypto.createDecipheriv("aes-256-gcm",key,Buffer.from(raw.iv,"base64"));decipher.setAAD(Buffer.from(id));decipher.setAuthTag(Buffer.from(raw.tag,"base64"));
const state=JSON.parse(Buffer.concat([decipher.update(Buffer.from(raw.data,"base64")),decipher.final()]).toString());
if(payload.action==="expire"){state.accessToken="expired-access-token-for-live-test";state.accessTokenExpiry=Date.now()-1000;state.checkedAt=0;}
if(payload.action==="check")state.checkedAt=0;
if(payload.action){const iv=crypto.randomBytes(12),cipher=crypto.createCipheriv("aes-256-gcm",key,iv);cipher.setAAD(Buffer.from(id));const data=Buffer.concat([cipher.update(JSON.stringify(state)),cipher.final()]);fs.writeFileSync(file,JSON.stringify({v:1,iv:iv.toString("base64"),tag:cipher.getAuthTag().toString("base64"),data:data.toString("base64")}),{mode:0o600});}
process.stdout.write(JSON.stringify({sessionId:state.sessionId,rememberId:id,revoked:!!state.revoked}));})().catch((error)=>{process.stderr.write("Remembered-session verification helper failed: "+error.message+"\\n");process.exit(1)});
`;
function stored(jar, action) {
  const parts = [...jar.cookies].filter(([name]) => name.includes("session-token")).sort(([a], [b]) => a.localeCompare(b));
  const cookie = parts.map(([, value]) => value).join("");
  return JSON.parse(execFileSync("docker", ["exec", "-i", "magistrala-ui", "node", "-e", helper], { input: JSON.stringify({ cookie, action }), encoding: "utf8" }));
}
function rotationCount(sessionId) {
  assert.match(sessionId, /^[a-f0-9-]{36}$/);
  return Number(execFileSync("docker", ["exec", "magistrala-atom-db", "psql", "-U", "atom", "-d", "atom", "-Atc", `SELECT count(*) FROM refresh_tokens WHERE session_id='${sessionId}' AND consumed_at IS NOT NULL;`], { encoding: "utf8" }).trim());
}
async function session(jar) { return (await request(jar, "/api/auth/session")).json(); }
async function logout(jar) {
  const csrf = await (await request(jar, "/api/auth/csrf")).json();
  await request(jar, "/api/auth/signout", { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: new URLSearchParams({ csrfToken: csrf.csrfToken, callbackUrl: base + "/login", json: "true" }) });
  testJars.delete(jar);
}
async function closeTestSessions() { for (const jar of testJars) await logout(jar).catch(() => {}); }
async function main() {
  const first = await signIn();
  console.log("PASS: official NextAuth login, persistent 400-day cookie, no browser refresh token");
  const record = stored(first.jar, "expire");
  const before = rotationCount(record.sessionId);
  const stale = first.jar.header();
  const results = await Promise.all(Array.from({ length: 16 }, async () => {
    const response = await fetch(base + "/api/auth/session", { headers: { Cookie: stale } });
    return response.json();
  }));
  for (const result of results) { assert.ok(result.accessToken); assert.equal(result.error, undefined); assert.equal(result.sessionUnavailable, undefined); }
  assert.equal(new Set(results.map((result) => result.accessToken)).size, 1);
  assert.equal(rotationCount(record.sessionId) - before, 1);
  console.log("PASS: expired access restores via refresh token; 16 concurrent requests rotate once");
  execFileSync("docker", ["restart", "magistrala-ui"], { stdio: "pipe" });
  let restarted;
  for (let i = 0; i < 12; i++) {
    try { restarted = await session(first.jar); break; } catch { await new Promise((resolve) => setTimeout(resolve, 1000)); }
  }
  assert.equal(restarted.accessToken, results[0].accessToken); assert.equal(restarted.error, undefined);
  console.log("PASS: UI container restart restores the persisted replacement with a stale browser cookie");
  await fetch("http://localhost:8080/graphql", { method: "POST", headers: { "Content-Type": "application/json", Authorization: "Bearer " + restarted.accessToken }, body: JSON.stringify({ query: "mutation{logout}" }) });
  stored(first.jar, "check");
  const revoked = await session(first.jar);
  assert.equal(revoked.error, "SessionRevoked"); assert.equal(revoked.accessToken, "");
  console.log("PASS: backend revocation prevents automatic recovery");
  await logout(first.jar);
  const second = await signIn();
  const secondRecord = stored(second.jar);
  const copied = new Jar(); copied.cookies = new Map(second.jar.cookies);
  await logout(second.jar);
  assert.deepEqual(await session(second.jar), {});
  assert.equal((await session(copied)).error, "SessionRevoked");
  const revokedAt = execFileSync("docker", ["exec", "magistrala-atom-db", "psql", "-U", "atom", "-d", "atom", "-Atc", `SELECT revoked_at IS NOT NULL FROM sessions WHERE id='${secondRecord.sessionId}';`], { encoding: "utf8" }).trim();
  assert.equal(revokedAt, "t");
  console.log("PASS: explicit sign-out clears the browser, revokes Atom, and rejects a copied cookie");
}
module.exports = { Jar, signIn, session, logout, stored, closeTestSessions };
if (require.main === module) main().catch(() => { console.error("Live remembered-login verification failed; no credentials were printed."); process.exitCode = 1; }).finally(closeTestSessions);
