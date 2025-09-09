# GO stream

<div align="center">
  <img src="build/appicon.png" alt="App Icon" width="128"/>
  <br/><br/>
  <p>一个简洁的桌面应用，用于启动您自己的直播推流服务。</p>
  <p>
    <img src="https://img.shields.io/badge/Wails-v2-red?style=for-the-badge&logo=wails" alt="Wails">
    <img src="https://img.shields.io/badge/Svelte-v3-orange?style=for-the-badge&logo=svelte" alt="Svelte">
    <img src="https://img.shields.io/badge/Go-1.18+-blue?style=for-the-badge&logo=go" alt="Go">
  </p>
</div>

---

## 📖 关于

GoStream 是一个基于 [LiveGo](https://github.com/gwuhaolin/livego) 开发的桌面应用程序，使用 [Wails](https://wails.io) 和 [Svelte](https://svelte.dev/) 构建。它旨在为个人主播或小型团队提供一个简单易用的图形化界面来管理直播推流。

本项目基于 `livego` 并集成了 `frpc` (内网穿透)，将核心服务打包成一个独立的执行文件，大大简化了部署和配置流程。现在，您只需要运行一个程序即可启动基础的直播服务。如果需要将直播流推送到公网，只需在程序同目录下放置一个 `frpc` 配置文件即可自动启用内网穿透功能。

## ⚙️ 核心组件

本应用基于以下强大的开源工具构建：

- **[LiveGo](https://github.com/gwuhaolin/livego)**: 一个简单、高效、纯 Go 语言实现的直播服务器。GoStream 基于 LiveGo 进行了二次开发，并将其作为核心推流引擎。
- **[FRP](https://github.com/fatedier/frp)**: 一个快速反向代理。已被集成到 GoStream 中，用于将您本地的直播服务安全地暴露到公网。


## 💡 如何使用

![App Screenshot](build/screenshot.png)

1.  打开 `GoStream.exe`。
2.  点击 **"Start"** 按钮来启动推流服务。如果检测到 `frpc` 配置文件，`frpc` 服务会一并启动。
3.  服务启动后，推流码会自动出现在界面上。
4.  (可选) 在 **“设置直播网页标题”** 输入框中填入您想要的标题，然后点击 **"Update"**。
5.  使用 OBS 或其他推流软件，从界面复制 **服务器地址** 和 **推流码** 并填入。
6.  开始推流！
7.  (可选) 如果需要将直播服务暴露到公网，请先参考下方的 **“内网穿透FRPC配置”** 部分完成配置。如果只在局域网使用，请忽略此步骤。
8.  直播结束后，点击 **"Stop"** 按钮。

## ⚙️ 内网穿透FRPC配置 (可选)

如果您需要将本地的直播服务暴露到公网，让任何人都能访问，则需要配置 FRP。如果只是在局域网内推流和观看，可以跳过此步骤。

**操作步骤**:

1.  在 **本应用 `GoStream.exe` 所在的根目录** 下，创建一个名为 `frpc.toml` 或 `frpc.ini` 的文件。
2.  填入您的 frp 服务器配置信息。本地网站端口为**7180**

下面是一个 `frpc.toml` 示例配置，它会将您本地的 `7180` 端口（网站服务端口）映射到 frp 服务器的某个指定端口（例如 `6000`）或某个网址(例如`gostream.com`)。

    ```toml
    # frpc.toml 示例
    serverAddr = "your_frp_server_ip"
    serverPort = "server_port"
    auth.token = "your_auth_token"

    [[proxies]]
    name = "your_name_tcp"
    type = "tcp"
    localIP = "127.0.0.1"
    localPort = 7180
    remotePort = 6000

    [[proxies]]
    name = "your_name_http"
    type = "http"
    localIP = "127.0.0.1"
    localPort = 7180
    customDomains = ["gostream.com"]

    ```
请将 `your_frp_server_ip`、`your_auth_token` 和 `remotePort` 替换为您自己的 frp 服务器信息。
注意: `GoStream` 的默认网站端口是 `7180`，RTMP 端口是 `1935`。请根据您的需要进行端口映射。

## ✨ 功能特性

- **单一服务控制**: 图形化界面一键控制所有后台服务 (`livego` 和 `frpc`) 的启停。
- **智能内网穿透**: 自动检测同目录下的 `frpc` 配置文件，存在即启动 `frpc` 服务。
- **自定义页面标题**: 在应用内直接修改直播网页的标题。
- **多语言支持**: 内置中/英文切换。
- **跨平台**: 基于 Wails 构建，可打包为 Windows, macOS 和 Linux 应用。

## 🔧 开发

如需进行二次开发，请确保您已安装 Go, Node.js 和 Wails。

### 实时开发

在项目根目录运行 `wails dev`。这将启动一个 Vite 开发服务器，提供前端的快速热重载。您也可以在浏览器中访问 `http://localhost:34115` 来调用 Go 的方法进行调试。

### 构建应用

运行 `wails build` 来构建一个可分发的、生产环境的软件包。

## 📜 授权协议

本项目基于 [MIT License](./LICENSE) 开源。
