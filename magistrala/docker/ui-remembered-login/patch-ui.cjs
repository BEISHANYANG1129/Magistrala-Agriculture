"use strict";
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const vm = require("node:vm");
const root = process.argv[2] || "/app/apps/community/.next";
const modulePath = "/app/remembered-login/remembered-session.cjs";
let authCount = 0;
for (const file of ["41170.js", "59139.js"]) {
  const target = path.join(root, "server/chunks", file);
  let source = fs.readFileSync(target, "utf8");
  function replace(before, after) {
    if (source.split(before).length !== 2) throw new Error(`${file}: expected one ${before.slice(0, 55)}; pinned UI changed`);
    source = source.replace(before, after);
  }
  replace("          verificationRequired\n        }\n      }`,{input:{identifier:a.username", "          verificationRequired\n          accessToken\n          accessTokenExpiresAt\n          refreshToken\n          refreshTokenExpiresAt\n        }\n      }`,{input:{identifier:a.username");
  replace("accessToken:c.token,accessTokenExpiry:new Date(c.expiresAt).getTime(),refreshToken:c.token,refreshTokenExpiry:new Date(c.expiresAt).getTime(),expires:c.expiresAt", "accessToken:c.accessToken,accessTokenExpiry:Date.parse(c.accessTokenExpiresAt),refreshToken:c.refreshToken,refreshTokenExpiry:Date.parse(c.refreshTokenExpiresAt),sessionId:c.sessionId,expires:c.accessTokenExpiresAt");
  replace("session:{maxAge:86400,updateAge:7200}", "session:{maxAge:34560000,updateAge:60},events:{async signOut(args){await require(" + JSON.stringify(modulePath) + ").signOut(args)}}");
  const start = source.indexOf("callbacks:{async jwt(");
  const end = source.indexOf("},pages:{", start);
  if (start < 0 || end < 0 || source.indexOf("callbacks:{async jwt(", start + 1) >= 0) throw new Error(`${file}: unknown auth callbacks`);
  source = source.slice(0, start) + "callbacks:{async jwt(args){return require(" + JSON.stringify(modulePath) + ").jwt(args)},async session(args){return require(" + JSON.stringify(modulePath) + ").session(args)}" + source.slice(end);
  replace("token:a,maxAge:86400", "token:await require(" + JSON.stringify(modulePath) + ").forEncoding(a),maxAge:34560000");
  if (!source.includes('secure:d,sameSite:"lax"}')) throw new Error(`${file}: workspace cookie helper changed`);
  source = source.replaceAll('secure:d,sameSite:"lax"}', 'secure:d,sameSite:"lax",maxAge:34560000}');
  replace("debug:!0", "debug:!1");
  fs.writeFileSync(target, source);
  authCount++;
}
const client = path.join(root, "static/chunks/app/[locale]/(app)/layout-d2d3cb7180ee5cd9.js");
let source = fs.readFileSync(client, "utf8");
const start = source.indexOf("function s(e){let{basePath:t,children:n}");
const end = source.indexOf("}},42075:", start);
if (start < 0 || end < 0) throw new Error("Unknown pinned client session watcher");
const watcher = `function s(e){let{basePath:t,children:n}=e,{update:o}=(0,l.useSession)(),busy=(0,i.useRef)(false);(0,i.useEffect)(()=>{let live=true;async function renew(){if(!live||busy.current||navigator.onLine===false||document.visibilityState==="hidden")return;busy.current=true;try{let session=await o();if(live&&session?.error==="SessionRevoked")(0,l.signOut)({callbackUrl:t+"login?error=SessionRevoked"})}catch{}finally{busy.current=false}}function visible(){if(document.visibilityState==="visible")renew()}renew();let timer=setInterval(renew,60000);window.addEventListener("focus",renew);window.addEventListener("online",renew);document.addEventListener("visibilitychange",visible);return()=>{live=false;clearInterval(timer);window.removeEventListener("focus",renew);window.removeEventListener("online",renew);document.removeEventListener("visibilitychange",visible)}},[o,t]);return n}`;
source = source.slice(0, start) + watcher + source.slice(end + 1);
new vm.Script(source);
// The upstream chunk is immutable in browser caches. Give the changed code its own content hash.
const oldName = path.basename(client);
const newName = "layout-" + crypto.createHash("sha256").update(source).digest("hex").slice(0, 16) + ".js";
fs.writeFileSync(path.join(path.dirname(client), newName), source);
fs.unlinkSync(client);
let references = 0;
function rewrite(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) { if (entry.name !== "cache") rewrite(filename); }
    else if (/\.(js|json|html|txt|rsc|meta)$/.test(entry.name)) {
      const value = fs.readFileSync(filename, "utf8");
      if (value.includes(oldName)) {
        fs.writeFileSync(filename, value.replaceAll(oldName, newName));
        references++;
      }
    }
  }
}
rewrite(root);
if (!references) throw new Error("Pinned client manifest did not reference the session watcher");
console.log(`Patched ${authCount} server auth bundles and the client session watcher (${references} cache-safe manifest references)`);
