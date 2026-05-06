module.exports = (md) => {
  const oldFence = md.renderer.rules.fence;

  md.renderer.rules.fence = (tokens, idx, options, env, slf) => {
    const old = oldFence(tokens, idx, options, env, slf);
    const preReg = /<pre><code[\w\s-="]*>/;

    if (preReg.exec(old)) {
      const pre = preReg.exec(old)[0];
      const post = "</code></pre>";
      return old.replace(pre, "").replace(post, "");
    }

    return old;
  };
};
