const MarkdownIt = require("markdown-it");
const markdownItDeflist = require("markdown-it-deflist");
const markdownItImplicitFigures = require("markdown-it-implicit-figures");
const markdownItTableOfContents = require("markdown-it-table-of-contents");
const markdownItRuby = require("markdown-it-ruby");

const headingSpan = require("./plugins/heading-span");
const imageFlow = require("./plugins/image-flow");
const linkFootnote = require("./plugins/link-footnote");
const listItemSection = require("./plugins/list-item-section");
const mathPlugin = require("./plugins/math");
const multiQuote = require("./plugins/multiquote");
const removePre = require("./plugins/remove-pre");
const tableContainer = require("./plugins/table-container");

function escapeAttribute(value = "") {
  return value.replace(/"/g, "&quot;");
}

function renderWechatFence(str, lang) {
  const text = str.replace(/</g, "&lt;").replace(/>/g, "&gt;");
  const lines = text.split("\n");
  const codeLines = [];
  const numbers = [];

  for (let i = 0; i < lines.length - 1; i += 1) {
    codeLines.push(`<code><span class="code-snippet_outer">${lines[i] || "<br>"}</span></code>`);
    numbers.push("<li></li>");
  }

  return (
    '<section class="code-snippet__fix code-snippet__js">' +
    '<ul class="code-snippet__line-index code-snippet__js">' +
    numbers.join("") +
    "</ul>" +
    `<pre class="code-snippet__js" data-lang="${escapeAttribute(lang || "")}">` +
    codeLines.join("") +
    "</pre></section>"
  );
}

function createWechatMarkdownParser() {
  const md = new MarkdownIt({
    html: true,
    highlight: renderWechatFence,
  });

  md.use(headingSpan)
    .use(removePre)
    .use(mathPlugin)
    .use(linkFootnote)
    .use(markdownItTableOfContents, {
      transformLink: () => "",
      includeLevel: [2, 3],
      markerPattern: /^\[toc\]/im,
    })
    .use(markdownItRuby)
    .use(markdownItImplicitFigures, {figcaption: true})
    .use(markdownItDeflist)
    .use(listItemSection)
    .use(imageFlow)
    .use(multiQuote)
    .use(tableContainer);

  return md;
}

module.exports = {
  createWechatMarkdownParser,
};
