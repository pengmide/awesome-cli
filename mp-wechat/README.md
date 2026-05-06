# mp-wechat

`mp-wechat` 是一个本地微信公众号文章 CLI 工具，统一提供两类能力：

- 将本地 Markdown 渲染成微信公众号可粘贴的 HTML 片段
- 将已有 HTML 片段上传到微信公众号草稿箱

项目维护目录：

```bash
/Users/pengmd/c/awesome-cli/mp-wechat
```

最终可编译出的二进制文件名：

```bash
mp-wechat-cli
```

## 安装

推荐使用 Bun：

```bash
cd /Users/pengmd/c/awesome-cli/mp-wechat
bun install
```

当前依赖：

- Bun
- macOS
- 系统自带 `swift`

`--copy-html` 会通过 `swift` 调用 macOS 的 `NSPasteboard`，把 HTML 写入系统剪贴板的 `text/html`。

## 开发模式

直接用 Bun 运行源码：

```bash
bun src/cli.js --help
```

渲染 Markdown：

```bash
bun src/cli.js ../a.md
```

上传已有 HTML 到公众号草稿箱：

```bash
bun src/cli.js publish-draft \
  ./out/clean_1_wechat_1770007948-edited.wechat.html \
  --appid "你的公众号AppID" \
  --secret "你的公众号AppSecret"
```

## 编译二进制

编译：

```bash
bun run build
```

构建脚本会临时处理 `css-tree` 在 Bun 单文件编译下的 JSON 读取问题，编译完成后会自动恢复 `node_modules` 中的原文件。

输出文件：

```bash
/Users/pengmd/c/awesome-cli/bin/mp-wechat-cli
```

使用二进制：

```bash
/Users/pengmd/c/awesome-cli/bin/mp-wechat-cli --help
/Users/pengmd/c/awesome-cli/bin/mp-wechat-cli ../a.md
/Users/pengmd/c/awesome-cli/bin/mp-wechat-cli publish-draft ./out/a.wechat.html --appid "xxx" --secret "yyy"
```

## 命令说明

### Markdown 转公众号 HTML

```bash
mp-wechat-cli <input.md> [--out <output.html>] [--theme <theme>] [--copy-html]
```

示例：

```bash
bun src/cli.js ../a.md --copy-html
```

默认输出到：

```text
out/<文件名>.wechat.html
```

当前只内置一个主题：

```text
green -> 绿意 / 夜尽天明
```

### HTML 上传到微信公众号草稿箱

```bash
mp-wechat-cli publish-draft <input.html> \
  --appid <appid> \
  --secret <secret> \
  [--out <output.html>] \
  [--title <title>] \
  [--author <author>] \
  [--digest <digest>]
```

示例：

```bash
bun src/cli.js publish-draft ./out/a.wechat.html \
  --appid "你的公众号AppID" \
  --secret "你的公众号AppSecret" \
  --title "自定义标题" \
  --author "作者名" \
  --digest "自定义摘要" \
  --out ./out/a.published.html
```

上传草稿时会执行：

- 获取微信公众号 `access_token`
- 将正文里的 `data:image/...;base64,...` 图片上传到微信 `media/uploadimg`
- 将 HTML 中的 base64 图片地址替换成微信返回的图片 URL
- 使用第一张 base64 图片上传永久素材，作为草稿封面
- 调用微信公众号草稿接口创建草稿
- 生成替换图片后的 `.published.html`

## 图片规则

Markdown 渲染阶段：

- `http://` 或 `https://` 图片会原样保留
- `data:image/...;base64,...` 图片会原样保留
- 本地相对路径或绝对路径图片不会自动上传

草稿上传阶段：

- 正文里的 base64 图片会上传到微信并替换成微信图片 URL
- 第一张 base64 图片会作为草稿封面图
- 如果 HTML 中没有 base64 图片，当前默认封面策略会报错

## 为什么不能直接用 pbcopy

`pbcopy` 通常只写入 `text/plain`，公众号后台会把 HTML 当普通文本处理，导致看到标签源码。

`--copy-html` 写入的是 `text/html`，公众号后台粘贴时可以按富文本解析。

## 实现说明

`src/cli.js` 是统一入口：

- 普通参数路径进入 Markdown 渲染流程
- `publish-draft` 子命令进入微信公众号草稿上传流程

上传草稿逻辑已经合并在 `src/cli.js` 中，项目不再单独维护 `src/wechat-publish.js`。
