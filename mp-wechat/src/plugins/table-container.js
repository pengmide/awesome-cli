function makeRule() {
  return function addTableContainer(state) {
    const nextTokens = [];

    for (let i = 0; i < state.tokens.length; i += 1) {
      const curToken = state.tokens[i];
      if (curToken.type === "table_open") {
        const tableContainerStart = new state.Token("html_inline", "", 0);
        tableContainerStart.content = '<section class="table-container">';
        nextTokens.push(tableContainerStart);
        nextTokens.push(curToken);
      } else if (curToken.type === "table_close") {
        const tableContainerClose = new state.Token("html_inline", "", 0);
        tableContainerClose.content = "</section>";
        nextTokens.push(curToken);
        nextTokens.push(tableContainerClose);
      } else {
        nextTokens.push(curToken);
      }
    }

    state.tokens = nextTokens;
  };
}

module.exports = (md) => {
  md.core.ruler.push("table-container", makeRule(md));
};
