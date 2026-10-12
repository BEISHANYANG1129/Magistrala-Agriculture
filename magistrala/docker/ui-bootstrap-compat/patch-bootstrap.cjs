"use strict";
// Staged proposal. Run only against a copy or during an approved image build.
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const root = process.argv[2];
if (!root) throw new Error("Explicit server/chunks directory required");
const specifications = [
  { file: "68163.js", paths: ["clients/configs", "clients/bootstrap"] },
  { file: "82268.js", paths: ["clients/configs", "clients/bootstrap"] },
  { file: "34897.js", paths: ["/clients/bootstrap/"] },
  { file: "81228.js", paths: ["/clients/bootstrap/"] },
];
const prepared = specifications.map(({ file, paths }) => {
  const target = path.join(root, file);
  let source = fs.readFileSync(target, "utf8");
  const counts = {};
  for (const original of paths) {
    counts[original] = source.split(original).length - 1;
    if (!counts[original]) throw new Error(`${file}: expected bootstrap path missing; inspect image version`);
    source = source.replaceAll(original, original.replace("clients", "devices"));
  }
  new vm.Script(source, { filename: file });
  return { target, source, file, counts };
});
for (const item of prepared) fs.writeFileSync(item.target, item.source);
console.log(JSON.stringify(prepared.map(({ file, counts }) => ({ file, counts })), null, 2));
