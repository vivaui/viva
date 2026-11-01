---
title: 构建与打包
summary: viva.toml、可复现的发布构建、开发期热重载，以及各桌面平台的安装包。
order: 12
group: distribute
---

`viva` 命令负责构建、打包和开发应用。它是开发工具：里面没有任何东西会链接进你的程序。

安装一次，然后在项目里运行。选适合你机器的方式：

| 环境 | 命令 |
| --- | --- |
| macOS、Linux | `brew install vivaui/tap/viva` |
| Windows | `scoop bucket add vivaui https://github.com/vivaui/homebrew-tap`，再 `scoop install viva` |
| 任意系统 | 从[发布页](https://github.com/vivaui/viva/releases)下载对应平台的压缩包，用 `SHA256SUMS` 校验，把 `viva` 放进 `PATH`。 |
| 已经在用 Go | `go install github.com/vivaui/viva/cmd/viva@latest` |

预编译压缩包以及 Homebrew、Scoop 包从 `v0.1.3` 起提供；`v0.1.2` 及更早版本请用 `go install`。

```sh
viva version           # 当前安装的版本
viva doctor            # 这台机器能不能构建？
viva help              # 所有命令
viva help package      # 某个命令的参数
```

### 不必先装 Go

`viva build`、`viva dev` 和 `viva init` 会调用 `go` 工具。找不到 Go，或者版本低于 Go 1.26 时，viva 会把官方工具链下载一次到用户缓存目录（macOS 是 `~/Library/Caches/viva/go`，Linux 是 `~/.cache/viva/go`，Windows 是 `%LocalAppData%\viva\go`），用 go.dev 公布的 SHA-256 校验，并且只给它自己的命令使用。下载前它会先告诉你。你的 `PATH` 和已经装好的 Go 都不会被改动。三个环境变量可以控制它：

- `VIVA_GO` 指向一个 `go` 可执行文件或 Go 安装目录，改用它。
- `VIVA_GO_DOWNLOAD=0` 禁止下载；此时 viva 会停下并给出说明。
- `VIVA_GO_MIRROR` 是代替 `https://go.dev/dl/` 的下载索引，例如 `https://golang.google.cn/dl/`。

下载和任何 Go 程序一样遵循 `HTTPS_PROXY`。macOS 上仍然需要 Xcode 命令行工具，Linux 上需要 XCB 和 Vulkan 库：这些是系统软件，viva 没法替你安装，但 `viva doctor` 会指出缺什么以及用哪条命令补上。

### 把 viva 固定在项目里

`viva add` 写出的配方随命令一起发布，所以命令和项目所依赖的 Viva 应该是同一个版本。`viva init` 因此会在 `go.mod` 里加一行 `tool`，`go tool viva` 运行的就是项目锁定的那个版本；升级 Viva 时命令一起升级，CI 也不需要另外安装：

```sh
go tool viva add --out ./components/ui button
go get -tool github.com/vivaui/viva/cmd/viva@v0.1.3   # 给已有项目锁定
```

`PATH` 里的命令和项目版本不一致时，`viva doctor` 和 `viva add` 会提醒。

## viva.toml

`viva build` 和 `viva package` 会读取 `go.mod` 旁边可选的 `viva.toml`：

```toml
[app]
name = "Notes"                 # 显示在菜单栏；也是 .app 的名字
id = "dev.example.notes"       # 包标识符
version = "0.1.0"
executable = "notes"           # 默认：名称的小写形式
package = "./cmd/notes"        # 要构建的 main 包
icon = "assets/icon.png"       # 正方形 PNG，至少 128 px（建议 1024），或 .icns

[web]
package = "."                      # 调用 app.RunCanvas 的 main 包也能编译到浏览器；应用用到仅限原生的代码时，改填另一个入口

[build]
tags = []                      # 所有目标共用的构建标签
out = "build"

[mac]
min_system = "12.0"            # 程序针对的 macOS 版本；默认值与下限都是 12.0
category = "public.app-category.productivity"
entitlements = "app.entitlements"   # 可选：连同 hardened runtime 一起签进应用包
```

文件按严格模式解析。未知的表或键、重复项、类型错误的值，都会报出带行号的错误，所以拼写错误不会悄悄改变构建结果。`[mac.dmg]` 决定 `.dmg` 打开时的窗口样子：`background`（PNG，2x 图片标 144 dpi；其点数尺寸就是窗口尺寸）、`icon_size`，以及 `app_position` / `applications_position`（`[x, y]`，从窗口左上角到图标中心的点数）。不设置时镜像以 540×380 的窗口打开，底色为浅色，并画有从应用指向 Applications 的箭头。另外还有用于额外 `Info.plist` 字符串的 `[mac.info]`，以及供安装包使用的 `[windows]`（发布者、安装范围、网址）和 `[linux]`（菜单分类、说明、维护者、依赖、关键字）。

## 发布构建

```sh
viva build                                  # build/ 里的 macOS 程序（默认目标 darwin、arm64）
viva build --target linux --arch amd64
viva build --target windows --arch amd64
viva build --target web                     # build/web/：程序、加载器、index.html
```

发布构建使用 `-trimpath`，去除符号并固定 build ID，所以相同的源码得到相同的字节。它会单独报告程序本身的大小，不含周围的其他东西。macOS 程序针对 `min_system` 编译，而不是针对构建机器自己的 macOS。Windows 发布构建使用 GUI 子系统；`--debug` 保留符号和控制台。

## 打包

```sh
viva package                                    # build/Notes.app
viva package --format app,zip,dmg
viva package --target windows --format nsis,zip
viva package --target linux --format tar.gz,deb
viva package --program ./notes-signed           # 打包你自己构建或签名的可执行文件
```

| 目标 | 格式 |
| --- | --- |
| macOS | `.app`、可复现的 `.zip`、`.dmg` |
| Windows | NSIS 安装程序（需要 `makensis`），或 `.zip` |
| Linux | 带安装脚本的 `tar.gz`，或 `.deb` |

**签名是可选的。** 什么都不配置就什么都不签：macOS 会阻止下载的应用包，Windows SmartScreen 会对安装程序给出警告。`--program` 的存在就是为了让你自己先构建、签名，再打包这个确定的可执行文件。

macOS 上 `viva package` 可以替你签名和公证。只有给了身份才签名，所有机密都来自环境变量，从不写进 `viva.toml` 或命令行：

| 变量 | 含义 |
| --- | --- |
| `APPLE_SIGNING_IDENTITY` | 用来签名的身份（`Developer ID Application: …` 或其哈希）；`-` 表示 ad hoc 签名，只有构建它的这台 Mac 认。`--sign-identity` 可覆盖 |
| `APPLE_CERTIFICATE`、`APPLE_CERTIFICATE_PASSWORD` | base64 的 `.p12`，导入到它自己的临时钥匙串，给 CI 机器用；没指定身份时就用其中的身份 |
| `APPLE_NOTARY_PROFILE` | 由 `xcrun notarytool store-credentials` 创建的配置 |
| `APPLE_API_KEY`、`APPLE_API_KEY_ID`、`APPLE_API_ISSUER` | App Store Connect API 密钥（`.p8` 的路径） |
| `APPLE_ID`、`APPLE_PASSWORD`、`APPLE_TEAM_ID` | Apple ID 与专用密码 |

应用包用 hardened runtime 和安全时间戳签名（外加 `[mac] entitlements` 或 `--entitlements`），然后验证。同时有公证凭据时：`.dmg` 会被签名、提交、等待结果并装订票据；没有 `.dmg` 的应用包则打成 zip 提交，票据装订到应用包上之后才压缩。被拒绝时会带着 Apple 给出的原因让这次运行失败。`--notarize` 让缺少凭据变成错误而不是跳过这一步，`--no-sign` 在已配置好的机器上跳过全部。

Windows 上，配置了证书就会用 Authenticode 给程序和安装程序签名，同样只从环境变量读取：`WINDOWS_CERTIFICATE_FILE`（`.pfx` 文件）或 `WINDOWS_CERTIFICATE`（同一个文件的 base64），配合 `WINDOWS_CERTIFICATE_PASSWORD`，以及 `WINDOWS_TIMESTAMP_URL`（RFC 3161 时间戳服务，默认 DigiCert）。签名由 `osslsigncode` 完成，它在任何系统上都能运行（`brew install osslsigncode`、`apt install osslsigncode`），Windows 上也可以用 `signtool`。密钥放在硬件令牌或云服务（例如 Azure Trusted Signing）里的证书无法成为文件：用 `--sign-command`（或 `WINDOWS_SIGN_COMMAND`）给一条原地给 `{file}` 签名的命令，例如 `--sign-command 'mysigner --file "{file}"'`。命令按空格拆成词（双引号用来成组），不经过 shell 直接运行。用 `--program` 提供的程序按原样使用，无论签没签；围绕它构建的安装程序仍会被签名。`--no-sign` 跳过全部。Linux 的包不签名。Apple 芯片上 Homebrew 版的 `makensis` 会崩溃，所以请在 Linux 或 Windows 上构建 Windows 安装程序。

## 热重载

```sh
viva dev ./cmd/notes
```

`viva dev` 在文件变化时重新构建，并替换工作进程，窗口和状态保持不变。热重载机制仅用于开发：发布构建不会链接它。
