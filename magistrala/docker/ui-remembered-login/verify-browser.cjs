"use strict";
const assert = require("node:assert/strict");
const path = require("node:path");
const fs = require("node:fs");
const { chromium } = require(path.resolve(__dirname, "../../../main/Magistrala/magistrala-ui/node_modules/playwright"));
const { Jar, signIn, logout, stored } = require("./verify-live.cjs");
const browserRoot = path.join(process.env.LOCALAPPDATA, "ms-playwright");
const release = fs.readdirSync(browserRoot).filter((name) => name.startsWith("chromium_headless_shell-")).sort().reverse()[0];
const executable = path.join(browserRoot, release, "chrome-headless-shell-win64/chrome-headless-shell.exe");
async function main() {
  const login = await signIn();
  let browser, context;
  const errors = [];
  function jarFrom(cookies) { const jar = new Jar(); jar.cookies = new Map(cookies.map((cookie) => [cookie.name, cookie.value])); return jar; }
  async function open(storageState) {
    browser = await chromium.launch({ executablePath: executable, headless: true });
    context = await browser.newContext({ storageState, viewport: { width: 1440, height: 1000 } });
    const page = await context.newPage();
    page.on("response", async (response) => {
      if (process.env.MG_REMEMBERED_TEST_INSPECT && response.request().headers()["next-action"]) {
        const text = await response.text().catch(() => "");
        console.log("INSPECT action:", response.status(), text.match(/"href":"([^"]+)"/)?.[1] || "no href");
      }
    });
    page.on("pageerror", (error) => errors.push(error.name + ": " + error.message));
    await page.goto("http://localhost:3000/en", { waitUntil: "domcontentloaded", timeout: 30000 });
    assert.ok(!new URL(page.url()).pathname.includes("login"), "Remembered browser must enter the application");
    return page;
  }
  try {
    const cookies = [...login.jar.cookies].map(([name, value]) => ({ name, value, domain: "localhost", path: "/", secure: false, httpOnly: name.includes("session-token") || name.includes("csrf-token"), sameSite: "Lax", expires: login.jar.expires.get(name) ? login.jar.expires.get(name) / 1000 : -1 }));
    let page = await open({ cookies, origins: [] });
    const session = await page.evaluate(() => fetch("/api/auth/session").then((response) => response.json()));
    assert.ok(session.user?.id); assert.equal(session.error, undefined);
    console.log("PASS: remembered browser opens the official application without a login prompt");
    const workspaceCard = page.getByRole("button").filter({ hasText: "Route:" }).first();
    await workspaceCard.waitFor({ state: "visible", timeout: 15000 });
    await workspaceCard.click();
    try { await page.waitForURL(/\/workspace\//, { waitUntil: "domcontentloaded", timeout: 20000 }); }
    catch (error) {
      const current = await page.evaluate(() => fetch("/api/auth/session").then((response) => response.json()));
      console.log("Workspace check:", { route: new URL(page.url()).pathname, workspace: current.workspace, error: current.error, deviceRetained: current.rememberId === session.rememberId });
      if (process.env.MG_REMEMBERED_TEST_INSPECT) console.log("INSPECT page:", (await page.locator("body").innerText()).slice(0, 2000));
      throw error;
    }
    const workspaceSession = await page.evaluate(() => fetch("/api/auth/session").then((response) => response.json()));
    assert.equal(workspaceSession.error, undefined);
    assert.equal(workspaceSession.rememberId, session.rememberId);
    assert.ok(workspaceSession.workspace?.id);
    for (const cookie of (await context.cookies()).filter((cookie) => cookie.name.includes("session-token"))) {
      assert.ok(cookie.expires * 1000 - Date.now() > 399 * 86400000, "Workspace selection must retain a persistent browser cookie");
      assert.equal(cookie.httpOnly, true);
    }
    console.log("PASS: selecting a workspace retains remembered login through cookie re-encoding");
    let state = await context.storageState();
    await browser.close();
    page = await open(state);
    assert.equal((await page.evaluate(() => fetch("/api/auth/session").then((response) => response.json()))).error, undefined);
    console.log("PASS: a new browser process restores the persisted browser cookie");
    stored(jarFrom(await context.cookies()), "expire");
    await page.reload({ waitUntil: "domcontentloaded", timeout: 30000 });
    const restored = await page.evaluate(() => fetch("/api/auth/session").then((response) => response.json()));
    assert.equal(restored.error, undefined); assert.equal(restored.sessionUnavailable, undefined); assert.ok(restored.accessToken);
    assert.ok(!new URL(page.url()).pathname.includes("login"));
    console.log("PASS: refreshing a page with expired access automatically restores login");
    await context.setOffline(true);
    stored(jarFrom(await context.cookies()), "expire");
    await page.evaluate(() => window.dispatchEvent(new Event("focus")));
    assert.ok(!new URL(page.url()).pathname.includes("login"));
    assert.ok((await context.cookies()).some((cookie) => cookie.name.includes("session-token")));
    await context.setOffline(false);
    await page.evaluate(() => window.dispatchEvent(new Event("online")));
    const reconnected = await page.evaluate(() => fetch("/api/auth/session").then((response) => response.json()));
    assert.equal(reconnected.error, undefined); assert.equal(reconnected.sessionUnavailable, undefined); assert.ok(reconnected.accessToken);
    console.log("PASS: browser offline/online transition preserves the device and restores expired access");
    assert.deepEqual(errors, [], "Browser must not have uncaught application errors");
    await logout(jarFrom(await context.cookies()));
    console.log("PASS: browser verification completed and its test session was signed out");
  } finally { await logout(login.jar).catch(() => {}); if (browser) await browser.close(); }
}
main().catch((error) => { console.error("Browser verification failed:", error.message.replace(/eyJ[A-Za-z0-9_.-]+/g, "[redacted]")); process.exitCode = 1; });
