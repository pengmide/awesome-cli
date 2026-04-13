# get-doubao-pic

一个本地 Go CLI，用来向豆包网页发送单个 prompt，等待生图完成，并把本轮生成的全部图片下载到本地。

它复用了当前目录里 [tampermonkey_doubao_pic.ts](/Users/pengmd/Work/QDM/IdeaProject/wechat-proxy/cli/get-doubao-pic/tampermonkey_doubao_pic.ts) 的核心思路，但把运行环境从浏览器脚本搬到了命令行：不做纯 HTTP 逆向，而是直接驱动真实 Chrome，抓取网页返回里的原图地址，再由 Go 下载保存。

## 适用场景

- 已经在本机使用 Chrome 登录过豆包，或者愿意在 CLI 打开的浏览器里登录一次
- 希望通过命令行自动发起一次生图任务
- 希望直接拿到原图并落盘到本地目录
- 不希望维护一套脆弱的纯 HTTP 逆向逻辑

## 当前能力边界

- 只支持单次 prompt
- 依赖本机已安装 Chrome 或 Chromium
- 依赖 Chrome 页面结构和豆包前端响应格式
- 优先从网络响应里提取 `image_ori_raw.url`
- 当网络响应体无法直接读取时，回退到页面内 `JSON.parse` hook
- 下载由 Go 完成，不依赖浏览器自带下载逻辑

## 实现原理

整体链路如下：

1. CLI 解析参数，构造运行配置。
2. 定位 Chrome 可执行文件。
3. 准备 Chrome 用户目录。
   - `copy`：复制 `Local State` 和指定 profile 到临时目录，适合一次性运行，但对豆包登录态只做“尽力继承”，不保证一定复用成功。
   - `direct`：直接使用原始 `user-data-dir`，要求原始 Chrome 已关闭。
   - `persistent`：使用 CLI 专用长期 `user-data-dir`。首次运行时会从原始 Chrome 完整初始化一份，后续不再重复复制，而是一直复用这份 CLI 自己维护的目录，适合“在 CLI 打开的浏览器里登录一次，以后长期复用”。
4. 使用 `chromedp` 启动并控制一个新的 Chrome 进程。
5. 打开 `https://www.doubao.com/chat/`。
6. 等待页面真正进入聊天态，再启发式定位输入框和发送按钮，自动输入 prompt 并发送。
7. 在生成过程中收集图片 URL。
   - 主路径：监听 `fetch` / `xhr` 网络响应，解析响应体中的 `creations` 和 `image_ori_raw.url`。
   - 回退路径：向页面注入 `JSON.parse` hook，把解析到的原图 URL 写入一个页面内收集器，再由 Go 轮询读取。
8. 使用稳定窗口判断本轮生成结束。
   如果已经抓到至少一张图，并且在一段时间内没有新图出现，则认为本轮结束。
9. 使用 Go 的 `http.Client` 下载图片到本地目录，处理文件名和重名冲突。

## 目录结构

```text
cli/get-doubao-pic/
├── main.go
├── go.mod
├── go.sum
├── README.md
├── tampermonkey_doubao_pic.ts
└── internal/
    ├── app/
    ├── browser/
    ├── config/
    ├── doubao/
    └── download/
```

主要模块职责：

- `main.go`
  程序入口。解析参数、启动主流程、输出结果和退出码。
- `internal/config`
  CLI 参数定义、默认值、路径和运行配置。
- `internal/app`
  组合整体流程：准备 session、执行抓图、下载结果、返回统一错误。
- `internal/browser`
  Chrome 启动、profile 准备、页面交互、网络监听、JSON hook。
- `internal/doubao`
  从响应数据中提取 `image_ori_raw.url`，管理图片收集状态和稳定窗口。
- `internal/download`
  下载图片、推导文件名、处理扩展名和重名。

## 运行依赖

- Go `1.25+`
- Chrome 或 Chromium
- 一个可用的豆包登录态

## 如何编译

进入当前目录：

```bash
cd /Users/pengmd/Work/QDM/IdeaProject/wechat-proxy/cli/get-doubao-pic
```

第一次编译前拉取依赖：

```bash
go mod tidy
```

本地编译：

```bash
go build -o ../bin/get-doubao-pic .
```

运行二进制：

```bash
./get-doubao-pic --prompt "画一只赛博朋克风格的猫"
```

如果不想先编译，也可以直接运行：

```bash
go run . --prompt "画一只赛博朋克风格的猫"
```

## 推荐使用方式

### 1. 最小命令

```bash
go run . --prompt "画一只赛博朋克风格的猫"
```

默认就会使用：

- `--profile-mode persistent`
- 默认的原始 Chrome 用户目录
- 默认的 CLI 专用长期 profile 目录
- `--timeout 180s`
- 自动生成的输出目录 `downloads/doubao-<timestamp>`

### 2. 推荐的长期使用方式

如果你希望显式写出完整参数，下面这条命令和默认行为一致：

```bash
go run . \
  --prompt "画一只赛博朋克风格的猫" \
  --profile-mode persistent \
  --profile-dir "$HOME/Library/Application Support/Google/Chrome" \
  --cli-profile-dir "$HOME/Library/Application Support/get-doubao-pic/chrome-user-data" \
  --output ./downloads/persistent-test \
  --timeout 180s
```

这里的 `--profile-dir` 是你原始 Chrome 的用户目录，`--cli-profile-dir` 是 CLI 自己长期复用的目录。

首次使用 `persistent` 时：

1. CLI 会初始化一个自己的 Chrome 用户目录。
2. 如果当前还没有豆包登录态，可以直接在 CLI 打开的浏览器里手动登录。
3. CLI 结束后，这个目录不会被删除。
4. 下次再次运行时，会继续复用这个 CLI 专用目录。

如果你想显式指定 CLI 专用目录，可以加上 `--cli-profile-dir`。

`persistent` 看起来像“不复制配置”，其实不是。它的真实行为是：

1. 第一次运行时，如果 `--cli-profile-dir` 不存在或者是空目录，CLI 会把原始 Chrome `user-data-dir` 整体复制一份到 `--cli-profile-dir`。
2. 从第二次开始，只要这个目录已经有内容，CLI 就不会再从原始 Chrome 同步，也不会再覆盖它。
3. 后续所有登录态、Cookie、Local Storage、IndexedDB、扩展状态，都是在这份 CLI 自己的目录里持续累积。

所以它稳定的原因，不是“它比 copy 更会复制”，而是“它只在第一次初始化时复制一次，之后就不再依赖原始 Chrome 的那份状态”。

### 3. 一次性运行

如果你只想做一次性运行，或者不想动 CLI 的长期 profile，可以显式使用 `copy`：

```bash
go run . \
  --prompt "画一只赛博朋克风格的猫" \
  --profile-mode copy \
  --profile-dir "$HOME/Library/Application Support/Google/Chrome" \
  --output ./downloads/copy-test \
  --timeout 240s
```

默认会自动读取 Chrome `Local State` 里的 `profile.last_used`，优先复用你当前最后活跃的 profile。
如果你需要固定某个 profile，比如 `Profile 1`，再显式传 `--profile-name "Profile 1"`。
如果这里依然没有继承到豆包登录态，这是 `copy` 模式的已知边界，改用 `persistent` 更稳。

### 4. 原始 profile 直接模式

如果你已经先关闭了 Chrome，也可以使用 `direct`：

```bash
go run . \
  --prompt "画一只赛博朋克风格的猫" \
  --profile-mode direct \
  --profile-dir "$HOME/Library/Application Support/Google/Chrome" \
  --profile-name "Default"
```

## 参数说明

- `--prompt`
  必填。要发给豆包的提问内容。
- `--output`
  可选。图片下载目录。
  未传时默认创建 `downloads/doubao-<timestamp>`。
- `--timeout`
  可选。整个生成流程的总超时时间。
- `--stable-window`
  可选。已经抓到图片后，用来判断“本轮生成是否结束”的稳定窗口。
- `--interactive-wait`
  可选。页面打开后等待可交互状态的最长时间。
- `--profile-dir`
  可选。原始 Chrome 用户目录路径。
  macOS 默认通常是 `~/Library/Application Support/Google/Chrome`。
- `--profile-name`
  可选。Chrome profile 名称。
  默认是 `auto`，会自动读取 Chrome `Local State` 中的 `profile.last_used`；
  如果读取失败，则回退到 `Default`。
- `--profile-mode`
  可选。支持 `copy`、`direct`、`persistent`。
  默认是 `persistent`。
- `--cli-profile-dir`
  可选。`persistent` 模式使用的 CLI 专用长期 profile 目录。
  默认路径：
  - macOS: `~/Library/Application Support/get-doubao-pic/chrome-user-data`
  - Linux: `~/.config/get-doubao-pic/chrome-user-data`
  - Windows: `%LOCALAPPDATA%\\get-doubao-pic\\chrome-user-data`
- `--chrome-path`
  可选。Chrome 可执行文件路径。
  当自动探测失败时手动指定。
- `--headless`
  可选。是否以无头模式运行。
  页面自动化站点对无头环境可能更敏感，建议单独验证。

## 输出结果

成功时，程序会输出：

- 下载成功的图片数量
- 输出目录
- 每张图片的保存路径

如果部分图片下载失败，还会额外输出 `warnings`。

## 如何测试

### 1. 静态测试

```bash
go test ./...
```

这会验证：

- 浏览器 profile 复制和持久化目录逻辑
- `image_ori_raw.url` 提取逻辑
- 基础模块是否可以正常编译

### 2. 真实 Smoke Test

推荐先测 `persistent`，这样你只需要在 CLI 浏览器里登录一次：

```bash
go run . \
  --prompt "画一只赛博朋克风格的猫" \
  --profile-mode persistent \
  --profile-dir "$HOME/Library/Application Support/Google/Chrome" \
  --cli-profile-dir "$HOME/Library/Application Support/get-doubao-pic/chrome-user-data" \
  --output ~/Tmp/doubao-pics \
  --timeout 240s
```

预期结果：

1. 自动启动一个受控 Chrome
2. 打开豆包聊天页
3. 如果需要，手动登录一次
4. 自动发送 prompt 或在登录后继续执行
5. 等待豆包出图
6. 在输出目录看到下载下来的图片

如果你测试 `copy` 模式时发现页面打开后仍然要求登录，或者 prompt 被写进页面但聊天态并未真正 ready，优先改用 `persistent`，让 CLI 自己持有一份长期可复用的登录态。

## 常见问题

### 1. 提示 Chrome profile 被占用

如果你使用的是 `direct` 模式，而本机 Chrome 还在运行，就可能遇到 profile 锁。

解决方式：

- 改用 `--profile-mode copy`
- 或改用 `--profile-mode persistent`
- 或者先关闭本机 Chrome，再使用 `direct`

### 2. 为什么 `persistent` 更稳

因为它和 `copy` 模式的“复制时机”完全不同：

- `copy`
  每次运行都从原始 Chrome 复制一份临时目录，运行结束后删除。
- `persistent`
  只在第一次初始化时复制一次，以后都直接使用 CLI 自己的长期目录。

这意味着：

- `copy` 每次都依赖“原始 Chrome 当前落盘的状态是不是正好可复用”
- `persistent` 一旦你在 CLI 打开的浏览器里登录成功，后续运行都沿着这份 CLI 自己的目录继续工作

所以 `persistent` 成功后，通常就不需要再依赖你日常使用的 Chrome 那份 profile 了。

### 3. 在 CLI 浏览器里登录过，但下次还是没登录

这通常说明你用的其实不是 `persistent`，而是 `copy`，或者换了 `--cli-profile-dir` 路径。

如果你希望登录能保留下来，确保用的是：

```bash
go run . \
  --prompt "画一只赛博朋克风格的猫" \
  --profile-mode persistent
```

并且两次运行时使用同一个 `--cli-profile-dir`。

### 4. 页面未登录或无法进入可交互状态

表现：

- 提示未登录
- 打开了豆包页面但无法进入可发送 prompt 的状态

排查方式：

- 先确认当前 profile 里能正常访问 `https://www.doubao.com/chat/`
- 优先使用 `persistent`，在 CLI 浏览器里手动登录一次
- 如果是 `direct` 模式，确认本机没有别的 Chrome 正在占用该目录

### 5. 抓不到图片

可能原因：

- prompt 没有成功提交
- 页面结构变化，输入框或发送按钮选择器没有命中
- 豆包响应结构变化，导致没有命中 `creations` / `image_ori_raw.url`

建议先用一个简单 prompt 做 smoke test，再结合终端错误继续排查。

### 6. `copy` 模式启动慢

这是预期行为，因为它需要先复制 `Local State` 和整个目标 profile。

profile 较大时，启动速度会明显慢于 `direct` 模式，但稳定性更好。

## 已知限制

- 只处理单次 prompt，不支持批量队列
- 依赖网页结构，页面大改版后可能需要更新选择器
- 依赖豆包返回结构，字段变化后需要更新提取逻辑
- 不做内置登录流程，`persistent` 也只是持久复用 CLI 自己的 profile
- 不承诺无头模式在所有环境下稳定可用

## 相关说明

当前目录中的 `tampermonkey_doubao_pic.ts` 是这个 CLI 的思路来源之一。它在浏览器环境里通过 hook 页面解析过程抓取图片地址；Go CLI 则把这条思路迁移到命令行里，通过浏览器自动化和网络监听完成同样的事情。
