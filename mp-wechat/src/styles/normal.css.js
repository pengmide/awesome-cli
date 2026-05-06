module.exports = `
#nice {
  font-size: 16px;
  color: black;
  padding: 0 10px;
  line-height: 1.6;
  word-spacing: 0;
  letter-spacing: 0;
  word-break: break-word;
  word-wrap: break-word;
  text-align: left;
  font-family: Optima-Regular, Optima, PingFangSC-light, PingFangTC-light, 'PingFang SC', Cambria, Cochin, Georgia, Times, 'Times New Roman', serif;
}

#nice p {
  font-size: 16px;
  padding-top: 8px;
  padding-bottom: 8px;
  margin: 0;
  line-height: 26px;
  color: black;
}

#nice h1,
#nice h2,
#nice h3,
#nice h4,
#nice h5,
#nice h6 {
  margin-top: 30px;
  margin-bottom: 15px;
  padding: 0;
  font-weight: bold;
  color: black;
}

#nice h1 { font-size: 24px; }
#nice h2 { font-size: 22px; }
#nice h3 { font-size: 20px; }
#nice h4 { font-size: 18px; }
#nice h5 { font-size: 16px; }
#nice h6 { font-size: 16px; }

#nice h1 .prefix,
#nice h2 .prefix,
#nice h3 .prefix,
#nice h4 .prefix,
#nice h5 .prefix,
#nice h6 .prefix {
  display: none;
}

#nice h1 .suffix,
#nice h2 .suffix,
#nice h3 .suffix,
#nice h4 .suffix,
#nice h5 .suffix,
#nice h6 .suffix {
  display: none;
}

#nice ul,
#nice ol {
  margin-top: 8px;
  margin-bottom: 8px;
  padding-left: 25px;
  color: black;
}

#nice ul { list-style-type: disc; }
#nice ul ul { list-style-type: square; }
#nice ol { list-style-type: decimal; }

#nice li section {
  margin-top: 5px;
  margin-bottom: 5px;
  line-height: 26px;
  text-align: left;
  color: rgb(1, 1, 1);
  font-weight: 500;
}

#nice blockquote {
  border: none;
}

#nice .multiquote-1 {
  display: block;
  font-size: 0.9em;
  overflow: auto;
  overflow-scrolling: touch;
  border-left: 3px solid rgba(0, 0, 0, 0.4);
  background: rgba(0, 0, 0, 0.05);
  color: #6a737d;
  padding: 10px 10px 10px 20px;
  margin-bottom: 20px;
  margin-top: 20px;
}

#nice .multiquote-1 p {
  margin: 0;
  color: black;
  line-height: 26px;
}

#nice .multiquote-2,
#nice .multiquote-3 {
  box-shadow: 1px 1px 10px rgba(0, 0, 0, 0.2);
  padding: 20px;
  margin-bottom: 20px;
  margin-top: 20px;
}

#nice .multiquote-3 p,
#nice .multiquote-3 h3 {
  text-align: center;
}

#nice .table-of-contents a {
  border: none;
  color: black;
  font-weight: normal;
}

#nice a {
  text-decoration: none;
  color: #1e6bb8;
  word-wrap: break-word;
  font-weight: bold;
  border-bottom: 1px solid #1e6bb8;
}

#nice strong {
  font-weight: bold;
  color: black;
}

#nice em,
#nice em strong,
#nice del {
  color: black;
}

#nice em,
#nice del {
  font-style: italic;
}

#nice hr {
  height: 1px;
  margin: 10px 0;
  border: none;
  border-top: 1px solid black;
}

#nice pre {
  margin-top: 10px;
  margin-bottom: 10px;
}

#nice pre code {
  display: -webkit-box;
  font-family: Operator Mono, Consolas, Monaco, Menlo, monospace;
  border-radius: 0;
  font-size: 12px;
  -webkit-overflow-scrolling: touch;
}

#nice pre code span {
  line-height: 26px;
}

#nice p code,
#nice li code {
  font-size: 14px;
  word-wrap: break-word;
  padding: 2px 4px;
  border-radius: 4px;
  margin: 0 2px;
  color: #1e6bb8;
  background-color: rgba(27, 31, 35, 0.05);
  font-family: Operator Mono, Consolas, Monaco, Menlo, monospace;
  word-break: break-all;
}

#nice img {
  display: block;
  margin: 0 auto;
  max-width: 100%;
}

#nice figure {
  margin: 10px 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}

#nice figcaption {
  margin-top: 5px;
  text-align: center;
  color: #888;
  font-size: 14px;
}

#nice .table-container {
  overflow-x: auto;
}

#nice table {
  display: table;
  text-align: left;
}

#nice tbody {
  border: 0;
}

#nice table tr {
  border: 0;
  border-top: 1px solid #ccc;
  background-color: white;
}

#nice table tr:nth-child(2n) {
  background-color: #f8f8f8;
}

#nice table tr th,
#nice table tr td {
  font-size: 16px;
  border: 1px solid #ccc;
  padding: 5px 10px;
  text-align: left;
}

#nice table tr th {
  font-weight: bold;
  background-color: #f0f0f0;
}

#nice table tr th:nth-of-type(n),
#nice table tr td:nth-of-type(n) {
  min-width: 85px;
}

#nice .footnote-word,
#nice .footnote-ref {
  color: #1e6bb8;
  font-weight: bold;
}

#nice .footnote-item {
  display: flex;
}

#nice .footnote-num {
  display: inline;
  width: 10%;
  background: none;
  font-size: 80%;
  opacity: 0.6;
  line-height: 26px;
  font-family: ptima-Regular, Optima, PingFangSC-light, PingFangTC-light, 'PingFang SC', Cambria, Cochin, Georgia, Times, 'Times New Roman', serif;
}

#nice .footnote-item p {
  display: inline;
  font-size: 14px;
  width: 90%;
  padding: 0;
  margin: 0;
  line-height: 26px;
  color: black;
  word-break: break-all;
  width: calc(100%-50);
}

#nice sub,
#nice sup {
  line-height: 0;
}

#nice .footnotes-sep:before {
  content: "参考资料";
  display: block;
}

#nice .block-equation {
  display: block;
  text-align: center;
  overflow: auto;
  -webkit-overflow-scrolling: touch;
}

#nice .block-equation svg {
  max-width: 300% !important;
  -webkit-overflow-scrolling: touch;
}

#nice .imageflow-layer1 {
  margin-top: 1em;
  margin-bottom: 0.5em;
  white-space: normal;
  border: 0 none;
  padding: 0;
  overflow: hidden;
}

#nice .imageflow-layer2 {
  white-space: nowrap;
  width: 100%;
  overflow-x: scroll;
}

#nice .imageflow-layer3 {
  display: inline-block;
  word-wrap: break-word;
  white-space: normal;
  vertical-align: middle;
  width: 100%;
}

#nice .imageflow-img {
  display: inline-block;
}

#nice .imageflow-caption {
  text-align: center;
  margin-top: 0;
  padding-top: 0;
  color: #888;
}

#nice figure a {
  display: flex;
  justify-content: center;
  align-items: center;
  border: none;
}

#nice figure a img {
  margin: 0;
}

#nice figure a + figcaption {
  display: flex;
  justify-content: center;
  align-items: center;
  width: 100%;
  margin-top: -35px;
  background: rgba(0, 0, 0, 0.7);
  color: white;
  line-height: 35px;
  z-index: 20;
}
`;
