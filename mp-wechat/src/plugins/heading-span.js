function slugify(value, md) {
  const spaceRegex = new RegExp(md.utils.lib.ucmicro.Z.source, "g");
  return encodeURIComponent(value.replace(spaceRegex, ""));
}

function makeRule(md, options) {
  return function addHeadingSpans(state) {
    for (let i = 0; i < state.tokens.length - 1; i += 1) {
      if (state.tokens[i].type !== "heading_open" || state.tokens[i + 1].type !== "inline") {
        continue;
      }

      const headingInlineToken = state.tokens[i + 1];
      if (!headingInlineToken.content) {
        continue;
      }

      if (options.addHeadingSpan) {
        const spanTokenPre = new state.Token("html_inline", "", 0);
        spanTokenPre.content = '<span class="prefix"></span><span class="content">';
        headingInlineToken.children.unshift(spanTokenPre);

        const spanTokenPost = new state.Token("html_inline", "", 0);
        spanTokenPost.content = '</span><span class="suffix"></span>';
        headingInlineToken.children.push(spanTokenPost);
      }

      i += 2;
    }
  };
}

module.exports = (md, opts) => {
  const defaults = {
    anchorClass: "markdown-it-headingspan",
    addHeadingSpan: true,
    slugify,
  };

  const options = md.utils.assign(defaults, opts);
  md.core.ruler.push("heading_span", makeRule(md, options));
};
