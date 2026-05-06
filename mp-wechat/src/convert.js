const fs = require("fs/promises");
const path = require("path");
const {JSDOM} = require("jsdom");
const juice = require("juice");

const {createWechatMarkdownParser} = require("./parser");

const PROJECT_ROOT = path.resolve(__dirname, "..");
const OUTPUT_DIR = path.join(PROJECT_ROOT, "out");

const BASIC_CSS = require("./styles/basic.css.js");
const NORMAL_CSS = require("./styles/normal.css.js");
const GREEN_THEME_CSS = require("../themes/green.css.js");

const THEMES = {
  green: {
    label: "绿意 / 夜尽天明",
    css: GREEN_THEME_CSS,
  },
};

function getThemeChoices() {
  return Object.keys(THEMES);
}

function escapeHtml(value) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function replaceMathMarkup(html) {
  return html
    .replace(/<mjx-container (class="inline.+?)<\/mjx-container>/g, "<span $1</span>")
    .replace(/\s<span class="inline/g, '&nbsp;<span class="inline')
    .replace(/svg><\/span>\s/g, "svg></span>&nbsp;")
    .replace(/mjx-container/g, "section")
    .replace(/class="mjx-solid"/g, 'fill="none" stroke-width="70"')
    .replace(/<mjx-assistive-mml.+?<\/mjx-assistive-mml>/g, "");
}

function buildShellHtml(parsedHtml, sourceName) {
  return `<!doctype html>
<html lang="zh-CN">
  <body>
    <div id="nice-rich-text-box" class="nice-wx-box">
      <section id="nice" data-tool="local_md" data-website="local_md" data-source="${escapeHtml(sourceName)}">
        ${parsedHtml}
      </section>
    </div>
  </body>
</html>`;
}

function collectWarnings(markdownText, articleNode) {
  const warnings = [];
  const imageNodes = Array.from(articleNode.querySelectorAll("img"));
  const localImageSources = imageNodes
    .map((node) => node.getAttribute("src") || "")
    .filter((src) => src && !src.startsWith("http://") && !src.startsWith("https://") && !src.startsWith("data:image/"));

  if (localImageSources.length > 0) {
    warnings.push(
      `Detected ${localImageSources.length} local image reference(s). They can render poorly or fail after pasting into WeChat unless you replace them with remote URLs or base64 images.`,
    );
  }

  if (/\$\$[\s\S]+?\$\$|\$(?!\s)[^$\n]+?\$/m.test(markdownText)) {
    warnings.push(
      "Detected LaTeX-style formulas. This local tool preserves the formula markup, but does not fully reproduce the website's MathJax-to-SVG copy path yet.",
    );
  }

  return warnings;
}

function inlineWechatFragment(parsedHtml, sourceName, themeCss) {
  const shell = new JSDOM(buildShellHtml(parsedHtml, sourceName));
  const {document} = shell.window;
  const articleNode = document.getElementById("nice");
  const directChildren = Array.from(articleNode.children);

  directChildren.forEach((child) => {
    child.setAttribute("data-tool", "local_md");
  });

  const fragmentNode = document.getElementById("nice-rich-text-box");
  const rawHtml = replaceMathMarkup(fragmentNode.innerHTML);
  const styleBundle = [BASIC_CSS, NORMAL_CSS, themeCss].join("\n\n");

  return {
    html: juice.inlineContent(rawHtml, styleBundle, {
      inlinePseudoElements: true,
      preserveImportant: true,
    }),
    articleNode,
  };
}

async function convertMarkdownFile({inputPath, outPath, theme = "green"}) {
  const themeEntry = THEMES[theme];
  if (!themeEntry) {
    throw new Error(`Unsupported theme "${theme}". Available themes: ${getThemeChoices().join(", ")}`);
  }

  const markdownText = await fs.readFile(inputPath, "utf8");
  const parser = createWechatMarkdownParser();
  const parsedHtml = parser.render(markdownText);
  const sourceName = path.basename(inputPath);

  const shell = new JSDOM(buildShellHtml(parsedHtml, sourceName));
  const warnings = collectWarnings(markdownText, shell.window.document.getElementById("nice"));
  const {html: fragmentHtml} = inlineWechatFragment(parsedHtml, sourceName, themeEntry.css);

  const finalOutPath =
    outPath || path.join(OUTPUT_DIR, `${path.parse(sourceName).name}.wechat.html`);

  await fs.mkdir(path.dirname(finalOutPath), {recursive: true});
  await fs.writeFile(finalOutPath, `${fragmentHtml.trim()}\n`, "utf8");

  return {
    outputPath: finalOutPath,
    themeLabel: themeEntry.label,
    warnings,
    html: `${fragmentHtml.trim()}\n`,
  };
}

module.exports = {
  convertMarkdownFile,
  getThemeChoices,
};
