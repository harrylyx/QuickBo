# QuickBo

桌面端的 TVBox 兼容播放器。界面用 [mygo](https://github.com/egoist/mygo) 的**原生 GPU UI**（`package ui`，Metal / Direct3D / OpenGL）绘制，不使用 WebView。

**本程序不内置任何片源、订阅地址或解析接口。** 安装之后是一个空壳，需要你自己填入配置 URL 或本地 JSON。

界面是深色影院布局：侧栏放源，首页是通栏横幅加 2:3 海报墙，详情页是海报加剧集。播放时侧栏收起，画面铺满窗口，控件会在鼠标停下后隐去。配色、圆角和阴影走 mygo 自己的主题接口。需要浅色时，在「配置」里切换。

## 能做什么

- 从 URL 或本地文件读取 TVBox / FongMi 风格的订阅 JSON（纯文本、带注释的宽松 JSON、Base64、`前缀**Base64`、gzip）。
- 点播：源切换、分类、海报墙、搜索、详情、剧集、播放。
- 直播：订阅里的 M3U /「名称,地址」TXT（含 `#genre#` 分组）。
- 仓库索引（`urls` / `storeHouse`）可以点进去继续加载下一层配置。
- 可以同时保存多份订阅地址：在「配置」里添加、停用或删除，侧栏按订阅分组，源合并到同一个首页。

## 源类型

| type | 形态 | v1 |
| --- | --- | --- |
| 0 | `api` 为 http(s) 的 MacCMS XML（`ac=list` / `videolist` / `detail`） | 支持。响应体是 JSON 时也会按 JSON 解析 |
| 1 | MacCMS JSON | 支持 |
| 3 | `api` 以 `.js` 结尾，或 ext 是 drpy 的 `rule` / `home`·`category`·`detail`·`search`·`play` | 部分支持。不是完整的 drpy2 模板引擎 |
| 3 | `csp_XBPQ`（ext 为规则 JSON 或规则 URL） | 支持常用截取字段（`分类`、`数组`、`标题`、`链接`、播放列表） |
| 3 | `csp_XYQHiker` / `csp_XYQHikerAL` | 部分支持：只认和 XBPQ 相同的截取字段 |
| 3 | 其它 `csp_*`、`.jar`、DEX | **不支持**。会在源列表里标成「不支持」，点开有说明，不会加载或执行 jar |
| 3 | `.py` | 不支持 |
| 0 但 api 不是 http 地址 | XPath 规则 | 不支持 |
| 4 | 远程站点 | 不当作点播源。如果 api 是 http(s)，可以手动「当作配置打开」 |

`vod_id` 允许数字或字符串。播放串按 `$$$` 分线路、`#` 分集、第一个 `$` 分集名和地址。

## 播放

优先用 mygo 的系统播放器（`github.com/egoist/mygo/ui/video`，随当前 mygo 主线提供；v0.8.3 标签还没有这个包，所以 go.mod 固定到包含它的提交）：

- MP4 / MOV 里的 H.264 + AAC：macOS、Windows、Linux 都可以，由系统解码器负责。
- HLS（`.m3u8`）：mygo 标明在 macOS、Windows 和浏览器可用。Linux 走 GStreamer，不保证。
- Linux 需要 GStreamer 1.x 和基础插件；H.264 还需要 `gstreamer1.0-libav`（或其它 H.264 解码插件）。没有这些库时，打开会返回「系统没有可播放视频的框架」。

播放地址如果不是直接的媒体文件，会先走配置里的 JSON 解析接口；没有的话再看页面里有没有 m3u8，或按 `/play/<id>/index.m3u8` 试一次。另外始终可以「用 mpv 播放」。mpv 会带上 Referer、User-Agent 和其它请求头；原生播放器的 `OpenURL` 只接受地址，带请求头的流如果失败，改用 mpv。设置里可以改成优先 mpv。系统播放器只报告「正在缓冲」，不提供已缓冲的时间范围，进度条上的浅色段只表示正在缓冲。

## 构建和运行

需要 Go 1.27.3 或更新。不要打开 cgo（mygo 本身也不用 cgo）。

```bash
go test ./...
CGO_ENABLED=0 go build -o quickbo ./cmd/quickbo
./quickbo
```

也可以在启动时添加一份配置（会并入已保存的订阅列表）：

```bash
./quickbo --config https://example.com/tvbox.json
./quickbo /path/to/tvbox.json
```

跨平台编译同样不需要 cgo：

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o quickbo.exe ./cmd/quickbo
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o quickbo ./cmd/quickbo
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o quickbo ./cmd/quickbo
```

订阅列表和偏好记在用户配置目录的 `quickbo/settings.json`，下次启动会加载全部启用的订阅。首次采集的分类与首页候选缓存在同目录的 `cache.json`；「最热影视」按各源首页候选的跨源重复数、清晰度和年份聚合。修改分类规则后点「保存」只用缓存重算，点「重新采集」才会更新片源内容。采集缓存格式升级时，旧缓存自动失效并重新采集一次。删掉 `settings.json` 就回到空壳。

## 参考与许可

解析层的划分（订阅解码、CMS、JS 爬虫、XBPQ 规则、直播列表）参考了 [UnBox](https://github.com/teaGod-s/UnBox)（MIT）。这里的实现是按这个结构另写的，没有整文件拷贝它的代码。

界面参考了 mygo 的文档和原生 UI 示例。没有使用 FongMi/TV、TVBoxOS、takagen99/Box、MisaTV、FreeBox 的代码。

本仓库以 MIT 许可发布，见 [LICENSE](LICENSE)。
