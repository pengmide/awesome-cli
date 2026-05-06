function renderFootnoteAnchorName(tokens, idx, options, env) {
  const n = Number(tokens[idx].meta.id + 1).toString();
  let prefix = "";

  if (typeof env.docId === "string") {
    prefix = `-${env.docId}-`;
  }

  return prefix + n;
}

function renderFootnoteCaption(tokens, idx) {
  let n = Number(tokens[idx].meta.id + 1).toString();

  if (tokens[idx].meta.subId > 0) {
    n += `:${tokens[idx].meta.subId}`;
  }

  return `[${n}]`;
}

function renderFootnoteWord(tokens, idx) {
  return `<span class="footnote-word">${tokens[idx].content}</span>`;
}

function renderFootnoteRef(tokens, idx, options, env, slf) {
  const caption = slf.rules.footnote_caption(tokens, idx, options, env, slf);
  return `<sup class="footnote-ref">${caption}</sup>`;
}

function renderFootnoteBlockOpen() {
  return '<h3 class="footnotes-sep"></h3>\n<section class="footnotes">\n';
}

function renderFootnoteBlockClose() {
  return "</section>\n";
}

function renderFootnoteOpen(tokens, idx, options, env, slf) {
  let id = slf.rules.footnote_anchor_name(tokens, idx, options, env, slf);

  if (tokens[idx].meta.subId > 0) {
    id += `:${tokens[idx].meta.subId}`;
  }

  return `<span id="fn${id}" class="footnote-item"><span class="footnote-num">[${id}] </span>`;
}

function renderFootnoteClose() {
  return "</span>\n";
}

function isSpace(code) {
  return code === 0x09 || code === 0x20;
}

function normalizeReference(str) {
  return str.trim().replace(/\s+/g, " ").toUpperCase();
}

function linkFoot(state, silent) {
  let attrs;
  let code;
  let label;
  let pos;
  let res;
  let ref;
  let title;
  let token;
  let href = "";
  let start = state.pos;
  let footnoteContent;
  let parseReference = true;
  const oldPos = state.pos;
  const max = state.posMax;

  if (state.src.charCodeAt(state.pos) !== 0x5b) {
    return false;
  }

  const labelStart = state.pos + 1;
  const labelEnd = state.md.helpers.parseLinkLabel(state, state.pos, true);

  if (labelEnd < 0) {
    return false;
  }

  pos = labelEnd + 1;
  if (pos < max && state.src.charCodeAt(pos) === 0x28) {
    parseReference = false;
    pos += 1;

    for (; pos < max; pos += 1) {
      code = state.src.charCodeAt(pos);
      if (!isSpace(code) && code !== 0x0a) {
        break;
      }
    }
    if (pos >= max) {
      return false;
    }

    start = pos;
    res = state.md.helpers.parseLinkDestination(state.src, pos, state.posMax);
    if (res.ok) {
      href = state.md.normalizeLink(res.str);
      footnoteContent = res.str;
      if (state.md.validateLink(href)) {
        pos = res.pos;
      } else {
        href = "";
      }
    }

    start = pos;
    for (; pos < max; pos += 1) {
      code = state.src.charCodeAt(pos);
      if (!isSpace(code) && code !== 0x0a) {
        break;
      }
    }

    res = state.md.helpers.parseLinkTitle(state.src, pos, state.posMax);
    if (pos < max && start !== pos && res.ok) {
      title = res.str;
      pos = res.pos;

      for (; pos < max; pos += 1) {
        code = state.src.charCodeAt(pos);
        if (!isSpace(code) && code !== 0x0a) {
          break;
        }
      }
    } else {
      title = "";
    }

    if (pos >= max || state.src.charCodeAt(pos) !== 0x29) {
      parseReference = true;
    }
    pos += 1;
  }

  if (parseReference) {
    if (typeof state.env.references === "undefined") {
      return false;
    }

    if (pos < max && state.src.charCodeAt(pos) === 0x5b) {
      start = pos + 1;
      pos = state.md.helpers.parseLinkLabel(state, pos);
      if (pos >= 0) {
        label = state.src.slice(start, pos);
        pos += 1;
      } else {
        pos = labelEnd + 1;
      }
    } else {
      pos = labelEnd + 1;
    }

    if (!label) {
      label = state.src.slice(labelStart, labelEnd);
    }

    ref = state.env.references[normalizeReference(label)];
    if (!ref) {
      state.pos = oldPos;
      return false;
    }
    href = ref.href;
    title = ref.title;
  }

  if (!silent) {
    if (title) {
      state.pos = labelStart;
      state.posMax = labelEnd;

      let tokens;

      if (!state.env.footnotes) {
        state.env.footnotes = {};
      }
      if (!state.env.footnotes.list) {
        state.env.footnotes.list = [];
      }

      const footnoteId = state.env.footnotes.list.length;
      state.md.inline.parse(`${title}: *${footnoteContent}*`, state.md, state.env, (tokens = []));

      token = state.push("footnote_word", "", 0);
      token.content = state.src.slice(labelStart, labelEnd);

      token = state.push("footnote_ref", "", 0);
      token.meta = {id: footnoteId};

      state.env.footnotes.list[footnoteId] = {tokens};
    } else {
      state.pos = labelStart;
      state.posMax = labelEnd;

      token = state.push("link_open", "a", 1);
      attrs = [["href", href]];
      token.attrs = attrs;
      if (title) {
        attrs.push(["title", title]);
      }

      state.md.inline.tokenize(state);
      state.push("link_close", "a", -1);
    }
  }

  state.pos = pos;
  state.posMax = max;
  return true;
}

function footnoteTail(state) {
  let current;
  let currentLabel;
  let insideRef = false;
  const refTokens = {};

  if (!state.env.footnotes) {
    return;
  }

  state.tokens = state.tokens.filter((tok) => {
    if (tok.type === "footnote_reference_open") {
      insideRef = true;
      current = [];
      currentLabel = tok.meta.label;
      return false;
    }
    if (tok.type === "footnote_reference_close") {
      insideRef = false;
      refTokens[`:${currentLabel}`] = current;
      return false;
    }
    if (insideRef) {
      current.push(tok);
    }
    return !insideRef;
  });

  if (!state.env.footnotes.list) {
    return;
  }

  const list = state.env.footnotes.list;

  state.tokens.push(new state.Token("footnote_block_open", "", 1));

  for (let i = 0; i < list.length; i += 1) {
    const item = list[i];
    state.tokens.push(
      Object.assign(new state.Token("footnote_open", "", 1), {
        meta: {id: i, label: item.label},
      }),
    );

    let tokens;
    if (item.tokens) {
      tokens = [];
      tokens.push(Object.assign(new state.Token("paragraph_open", "p", 1), {block: true}));
      tokens.push(Object.assign(new state.Token("inline", "", 0), {children: item.tokens, content: ""}));
      tokens.push(Object.assign(new state.Token("paragraph_close", "p", -1), {block: true}));
    } else if (item.label) {
      tokens = refTokens[`:${item.label}`];
    } else {
      tokens = [];
    }

    state.tokens = state.tokens.concat(tokens);
    if (state.tokens[state.tokens.length - 1].type === "paragraph_close") {
      const lastParagraph = state.tokens.pop();
      state.tokens.push(lastParagraph);
    }

    state.tokens.push(new state.Token("footnote_close", "", -1));
  }

  state.tokens.push(new state.Token("footnote_block_close", "", -1));
}

module.exports = (md) => {
  md.renderer.rules.footnote_ref = renderFootnoteRef;
  md.renderer.rules.footnote_word = renderFootnoteWord;
  md.renderer.rules.footnote_block_open = renderFootnoteBlockOpen;
  md.renderer.rules.footnote_block_close = renderFootnoteBlockClose;
  md.renderer.rules.footnote_open = renderFootnoteOpen;
  md.renderer.rules.footnote_close = renderFootnoteClose;
  md.renderer.rules.footnote_caption = renderFootnoteCaption;
  md.renderer.rules.footnote_anchor_name = renderFootnoteAnchorName;

  md.inline.ruler.at("link", linkFoot);
  md.core.ruler.after("inline", "footnote_tail", footnoteTail);
};
