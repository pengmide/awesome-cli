function makeRule() {
  return function addBlockQuoteClass(state) {
    let count = 0;
    let outerQuoteToken;

    for (let i = 0; i < state.tokens.length; i += 1) {
      const curToken = state.tokens[i];
      if (curToken.type === "blockquote_open") {
        if (count === 0) {
          outerQuoteToken = curToken;
        }
        count += 1;
        continue;
      }

      if (count > 0) {
        outerQuoteToken.attrs = [["class", `multiquote-${count}`]];
        count = 0;
      }
    }
  };
}

module.exports = (md) => {
  md.core.ruler.push("blockquote-class", makeRule(md));
};
