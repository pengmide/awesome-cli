#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const {spawnSync} = require("child_process");

const root = path.resolve(__dirname, "..");
const outputPath = path.resolve(root, "..", "bin", "mp-wechat-cli");
const files = {
  cjsPatch: path.join(root, "node_modules/css-tree/cjs/data-patch.cjs"),
  esmPatch: path.join(root, "node_modules/css-tree/lib/data-patch.js"),
  cjsVersion: path.join(root, "node_modules/css-tree/cjs/version.cjs"),
  esmVersion: path.join(root, "node_modules/css-tree/lib/version.js"),
};

function read(filePath) {
  return fs.readFileSync(filePath, "utf8");
}

function write(filePath, content) {
  fs.writeFileSync(filePath, content, "utf8");
}

function run(command, args) {
  const result = spawnSync(command, args, {
    cwd: root,
    stdio: "inherit",
  });

  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    process.exit(result.status || 1);
  }
}

const originals = Object.fromEntries(
  Object.entries(files).map(([key, filePath]) => [key, read(filePath)]),
);

try {
  const patch = JSON.stringify(require(path.join(root, "node_modules/css-tree/data/patch.json")));
  const {version} = require(path.join(root, "node_modules/css-tree/package.json"));

  // Bun's single-file compiler misses these css-tree JSON requires, so inline
  // them only for the compile step and restore node_modules immediately after.
  write(files.cjsPatch, `'use strict';\n\nconst patch = ${patch};\n\nmodule.exports = patch;\n`);
  write(files.esmPatch, `const patch = ${patch};\n\nexport default patch;\n`);
  write(files.cjsVersion, `'use strict';\n\nexports.version = ${JSON.stringify(version)};\n`);
  write(files.esmVersion, `export const version = ${JSON.stringify(version)};\n`);

  fs.mkdirSync(path.dirname(outputPath), {recursive: true});
  run("bun", ["build", "src/cli.js", "--compile", "--outfile", outputPath]);
} finally {
  Object.entries(files).forEach(([key, filePath]) => {
    write(filePath, originals[key]);
  });
}
