#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const {spawnSync} = require("child_process");

const root = path.resolve(__dirname, "..");
const outputPath = path.resolve(root, "..", "bin", "mp-wechat-cli");
const files = {
  cjsData: path.join(root, "node_modules/css-tree/cjs/data.cjs"),
  cjsPatch: path.join(root, "node_modules/css-tree/cjs/data-patch.cjs"),
  esmData: path.join(root, "node_modules/css-tree/lib/data.js"),
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
    throw new Error(`${command} ${args.join(" ")} failed with exit code ${result.status || 1}`);
  }
}

function jsonExpression(value) {
  const json = JSON.stringify(value)
    .replace(/\u2028/g, "\\u2028")
    .replace(/\u2029/g, "\\u2029");
  const base64 = Buffer.from(json, "utf8").toString("base64");
  const chunks = base64.match(/.{1,120}/g) || [];
  return `JSON.parse(Buffer.from(${JSON.stringify(chunks)}.join(""), "base64").toString("utf8"))`;
}

const originals = Object.fromEntries(
  Object.entries(files).map(([key, filePath]) => [key, read(filePath)]),
);

try {
  const patch = jsonExpression(require(path.join(root, "node_modules/css-tree/data/patch.json")));
  const mdnAtrules = jsonExpression(require(path.join(root, "node_modules/mdn-data/css/at-rules.json")));
  const mdnProperties = jsonExpression(require(path.join(root, "node_modules/mdn-data/css/properties.json")));
  const mdnSyntaxes = jsonExpression(require(path.join(root, "node_modules/mdn-data/css/syntaxes.json")));
  const {version} = require(path.join(root, "node_modules/css-tree/package.json"));

  // Bun's single-file compiler misses these css-tree JSON requires, so inline
  // them only for the compile step and restore node_modules immediately after.
  write(
    files.cjsData,
    originals.cjsData.replace(
      [
        "const mdnAtrules = require('mdn-data/css/at-rules.json');",
        "const mdnProperties = require('mdn-data/css/properties.json');",
        "const mdnSyntaxes = require('mdn-data/css/syntaxes.json');",
      ].join("\n"),
      [
        `const mdnAtrules = ${mdnAtrules};`,
        `const mdnProperties = ${mdnProperties};`,
        `const mdnSyntaxes = ${mdnSyntaxes};`,
      ].join("\n"),
    ),
  );
  write(
    files.esmData,
    originals.esmData.replace(
      [
        "import { createRequire } from 'module';",
        "import patch from './data-patch.js';",
        "",
        "const require = createRequire(import.meta.url);",
        "const mdnAtrules = require('mdn-data/css/at-rules.json');",
        "const mdnProperties = require('mdn-data/css/properties.json');",
        "const mdnSyntaxes = require('mdn-data/css/syntaxes.json');",
      ].join("\n"),
      [
        "import patch from './data-patch.js';",
        "",
        `const mdnAtrules = ${mdnAtrules};`,
        `const mdnProperties = ${mdnProperties};`,
        `const mdnSyntaxes = ${mdnSyntaxes};`,
      ].join("\n"),
    ),
  );
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
