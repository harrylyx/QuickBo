<h1 align="center">QuickBo</h1>

<p align="center">
  跨平台 <b>TVBox 兼容</b>桌面播放器：点播 + 直播，原生界面，不用 WebView。
</p>

<p align="center">
  <a href="https://github.com/harrylyx/QuickBo/releases"><img src="https://img.shields.io/github/v/release/harrylyx/QuickBo?label=下载&color=2ea44f" alt="下载" /></a>
  <a href="https://github.com/harrylyx/QuickBo/actions/workflows/test.yml"><img src="https://github.com/harrylyx/QuickBo/actions/workflows/test.yml/badge.svg" alt="Test" /></a>
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/UI-mygo%20原生-8A2BE2" alt="mygo" />
  <img src="https://img.shields.io/badge/平台-macOS%20%7C%20Windows%20%7C%20Linux-blue" alt="平台" />
  <img src="https://img.shields.io/badge/License-MIT-green" alt="License" />
</p>

QuickBo 用 [mygo](https://github.com/egoist/mygo) 的原生 GPU 界面（Metal / Direct3D / OpenGL）绘制，视频由 libmpv 渲染在窗口里。

**本程序不内置任何片源、订阅地址或解析接口。** 装好后是一个空壳，需要你自己添加 TVBox / FongMi 订阅地址或本地 JSON。

## ✨ 特性

- **多订阅合并**：同时添加多份订阅，可启用、停用、批量管理；各订阅的片源合并到一个首页。
- **聚合首页**：通栏横幅 + 海报墙。按你设定的分类（电影、电视剧、地区等，可拖动排序、导入导出）汇总各片源内容，同一部片跨源合并；「查看更多」进入滚动加载的汇总页。
- **全源搜索**：首页一次搜索全部片源，结果按片名合并。
- **片源管理**：测速、健康度、按订阅/类型分组、停用/启用、黑名单（支持批量粘贴关键字）。
- **详情页换源**：同一部片列出所有片源的线路，自动标出无法播放的线路。
- **播放器**
  - 窗口内播放（libmpv），无边框**悬浮小窗**，全屏
  - 按住空格或 B 临时 2 倍速，倍速 0.5×–3×
  - 跳过片头片尾（按剧记住）、自动播放下一集（带倒计时）
  - 字幕切换、外挂字幕、字幕大小与延迟、音轨切换
  - 画面比例、亮度对比度、截图
  - 音量 / 倍速 / 快进的屏幕提示，滚轮调音量，双击全屏
- **观看历史与收藏**：断点续播，首页「继续观看」。
- **直播**：M3U 与「名称,地址」TXT（含 `#genre#` 分组）。
- **外观**：跟随系统 / 浅色 / 深色。

## 📡 支持的源

| type | 形态 | 支持情况 |
| --- | --- | --- |
| 0 | MacCMS XML（`api` 为 http(s)） | 支持；响应是 JSON 时按 JSON 解析 |
| 1 | MacCMS JSON | 支持 |
| 3 | `.js` / drpy `rule` | 部分支持（不是完整的 drpy2 模板引擎） |
| 3 | `csp_XBPQ` | 支持常用截取字段 |
| 3 | `csp_XYQHiker` / `csp_XYQHikerAL` | 部分支持（与 XBPQ 相同的字段） |
| 3 | 其它 `csp_*`、`.jar`、DEX、`.py` | **不支持**，片源列表里标为「不支持」，不会加载或执行 |
| 4 | 远程站点 | 不作为点播源；http(s) 地址可手动「当作配置打开」 |

订阅格式：纯 JSON、带注释的宽松 JSON、Base64、`前缀**Base64`、gzip；支持仓库索引（`urls` / `storeHouse`）逐层打开。

## 🖥️ 安装

前往 [Releases](https://github.com/harrylyx/QuickBo/releases) 下载对应平台的安装包：

| 平台 | 安装包 | 播放所需 |
| --- | --- | --- |
| macOS | `quickbo-<版本>-arm64.dmg`（Apple 芯片）/ `-amd64.dmg`（Intel） | `brew install mpv` |
| Windows | `quickbo-setup-<版本>-amd64.exe` / `-arm64.exe` | 把 `libmpv-2.dll` 放进安装目录（见下） |
| Linux | `quickbo_<版本>_amd64.deb` / `_arm64.deb`，或 `.tar.gz`（内含 `install.sh`） | `.deb` 自动依赖 `libmpv2`；`.tar.gz` 需 `sudo apt install libmpv2` |

> 💡 **关于 libmpv**：QuickBo 用 libmpv 在窗口里解码播放，安装包本身不带它。
> - macOS：`brew install mpv`。
> - Windows：从 [mpv-player-windows/libmpv](https://sourceforge.net/projects/mpv-player-windows/files/libmpv/) 下载，把 `libmpv-2.dll` 放到 `%LOCALAPPDATA%\Programs\QuickBo`（程序所在目录）。
> - Linux：`sudo apt install libmpv2`。
>
> 没有 libmpv 但装了 mpv 程序时，会改用独立的 mpv 窗口播放（此时没有小窗、字幕面板等窗口内功能），并说明原因；两者都没有时，播放页会提示安装方法。

> ⚠️ **首次打开提示**：安装包暂未做开发者签名。
> - **macOS** 提示「无法验证开发者」时：在「应用程序」里右键 QuickBo →「打开」，或执行
>   `xattr -dr com.apple.quarantine /Applications/QuickBo.app`。
> - **Windows** SmartScreen 提示「已保护你的电脑」时：点「更多信息」→「仍要运行」。

**系统要求**：macOS 12+；Windows 10+；Linux 需 GTK 3（`.deb` 自动声明依赖）。

## 🚀 使用

1. 打开后点「添加订阅」，粘贴配置 URL，或「打开本地文件」选一个 JSON。
2. **首页**浏览各分类、横幅推荐，或在顶部搜索全部片源。
3. 点海报进详情页，选线路和剧集播放；**历史**、**收藏**在侧栏「资料库」。
4. 侧栏 **管理**：订阅、片源（测速 / 停用 / 黑名单）、首页分类（拖动排序、导入导出）。
5. 侧栏底部的 **设置** 切换外观。

订阅和偏好保存在用户配置目录的 `quickbo/settings.json`（macOS 为 `~/Library/Application Support/quickbo/`），采集缓存在同目录的 `cache.json`。删掉 `settings.json` 就回到空壳。

### ⌨️ 播放快捷键

| 按键 | 作用 |
| --- | --- |
| 空格 | 播放 / 暂停 |
| 按住空格 或 按住 B | 临时 2 倍速，松开恢复 |
| ← / → | 后退 / 快进 10 秒（Shift：60 秒） |
| ↑ / ↓ 或滚轮 | 音量 |
| M | 静音 |
| [ / ] / \ | 减速 / 加速 / 恢复 1 倍速 |
| N 或 PageDown / PageUp | 下一集 / 上一集 |
| F 或双击画面 | 全屏 |
| P | 悬浮小窗 |
| S | 截图（保存到「图片/QuickBo」） |
| Esc | 退出全屏 / 返回 |
| ? | 显示全部快捷键 |

## 🔧 开发者构建

需要 Go 1.27.3+，不需要 cgo。打包工具 mygo CLI 已固定在 `go.mod` 里，配置见 `mygo.json`。

```bash
go run ./cmd/quickbo                 # 直接运行
go test ./...                        # 测试
go tool mygo build                   # 打包当前平台到 build/
go tool mygo build -platform linux/amd64,windows/amd64 -skip-installer   # 交叉编译
```

启动时也可以带一份配置（会并入已保存的订阅）：

```bash
go run ./cmd/quickbo --config https://example.com/tvbox.json
```

libmpv 的真实播放测试默认跳过，装好 libmpv 后可以这样跑：

```bash
QUICKBO_LIBMPV_TEST=1 go test ./internal/player/mpvlib
```

## 📦 发布

由 [GitHub Actions](.github/workflows/release.yml) 完成：

1. 修改 `mygo.json` 的 `version`，在 `CHANGELOG.md` 写上这一版的说明，提交。
2. 推送同名标签：

   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. 工作流检查标签与 `mygo.json` 版本一致，分别打包 macOS（arm64 / amd64）、Windows（amd64 / arm64）、Linux（amd64 / arm64），然后创建 Release 并上传安装包。

在 Actions 页手动运行 Release 工作流，只打包不发布，可用来检查各平台能否打包。每次推送和 PR 会由 [Test](.github/workflows/test.yml) 在 Linux 和 macOS 上编译并跑测试。

## ⚠️ 已知限制

- 不执行 JAR / DEX 爬虫与 Python 爬虫；drpy 只支持常见写法。
- 安装包不带 libmpv，需按上文另行安装。
- 安装包未签名，首次打开需按上文放行。
- 播放地址需要会员或平台页面（B站、腾讯视频等）时，依赖订阅里配置的解析接口。

## 🙏 参考与许可

解析层的划分（订阅解码、CMS、JS 爬虫、XBPQ 规则、直播列表）参考了 [UnBox](https://github.com/teaGod-s/UnBox)（MIT），实现是按这个结构另写的，没有整文件拷贝它的代码。打包与发布流程参考了 [mygo-music](https://github.com/egoist/mygo-music) 和 mygo 文档。

没有使用 FongMi/TV、TVBoxOS、takagen99/Box、MisaTV、FreeBox 的代码。

本仓库以 MIT 许可发布，见 [LICENSE](LICENSE)。
