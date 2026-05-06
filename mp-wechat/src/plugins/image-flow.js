const defaultOption = {
  limitless: false,
  limit: 10,
};

function escapeAttribute(value = "") {
  return value.replace(/"/g, "&quot;");
}

module.exports = (md, opt) => {
  const options = opt || defaultOption;

  const tokenize = (state, start) => {
    let token;

    const matchReg = /^<((!\[[^[\]]*\]\([^()]+\)(,?\s*(?=>)|,\s*(?!>)))+)>/;
    const srcLine = state.src.slice(state.bMarks[start], state.eMarks[start]);

    if (srcLine.charCodeAt(0) !== 0x3c) {
      return false;
    }
    const match = matchReg.exec(srcLine);

    if (match) {
      const images = match[1].match(/\[[^\]]*\]\([^)]+\)/g);
      if (!options.limitless && images.length <= options.limit) {
        token = state.push("imageFlow", "", 0);
        token.meta = images;
        token.block = true;
        state.line += 1;
        return true;
      }
    }
    return false;
  };

  md.renderer.rules.imageFlow = (tokens, idx) => {
    const start = '<section class="imageflow-layer1"><section class="imageflow-layer2">';
    const end = '</section></section><p class="imageflow-caption"><<< 左右滑动见更多 >>></p>';
    const contents = tokens[idx].meta;
    let wrappedContent = "";

    contents.forEach((content) => {
      const altMatch = content.match(/\[([^[\]]*)\]/);
      const srcMatch = content.match(/[^[]*\(([^()]*)\)[^\]]*/);
      const alt = altMatch ? altMatch[1] : "";
      const src = srcMatch ? srcMatch[1] : "";
      wrappedContent += `<section class="imageflow-layer3"><img alt="${escapeAttribute(
        alt,
      )}" src="${escapeAttribute(src)}" class="imageflow-img" /></section>`;
    });

    return start + wrappedContent + end;
  };

  md.block.ruler.before("paragraph", "imageFlow", tokenize);
};
