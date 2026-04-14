# Auto Rename Service (Go)

一个 Go 后台服务：监听 `/home/home/openlist/file` 下的一级子目录，当有新图片文件进入时，自动重命名为：

`<父目录名><YYYYMMDD>-<序号><原扩展名>`

例如目录 `123` 在 2026-03-23 新增 `abc.jpg`，会重命名为：

`12320260323-1.jpg`

## 功能说明

- 只监听根目录下的一级子目录（并监听根目录用于动态发现新一级子目录）
- 只处理运行后新增的文件，不启动时处理历史文件
- 只处理图片扩展名（大小写不敏感）：
  - `.jpg .jpeg .png .webp .gif .bmp .tiff .heic .heif`
- 自动跳过：
  - 隐藏文件（`.` 开头）
  - 系统文件（`.DS_Store`）
  - 临时前缀（`~$`、`._`）
  - 临时扩展名（`.tmp .temp .part .crdownload .download .partial .filepart .swp .swo`）
- 已符合当天命名规则的文件会跳过
- 文件稳定检测：
  - `EventDelay = 3s`
  - `FileStableChecks = 4`
  - `FileStableInterval = 1s`
- 并发安全：
  - 目录级加锁，同目录串行，不同目录并行
- 支持事件：
  - `create`
  - `rename / move into directory`（由 fsnotify 事件触发）
- 运行中新增一级子目录会自动加入监听

## 依赖安装

要求：

- Go 1.22+
- Linux（建议 systemd 管理服务）

下载依赖：

```bash
go mod tidy
```

## 运行方法

直接运行：

```bash
go run .
```

默认监听目录：

`/home/home/openlist/file`

## 编译方法

```bash
go build -o auto_rename .
```

编译后执行：

```bash
./auto_rename
```

## systemd 使用方法

1. 复制二进制到目标目录（示例）

```bash
mkdir -p /home/home/openlist/auto_rename
cp auto_rename /home/home/openlist/auto_rename/
```

2. 复制服务文件

```bash
cp auto-rename.service /etc/systemd/system/
```

3. 重载并启用

```bash
systemctl daemon-reload
systemctl enable --now auto-rename.service
```

4. 查看状态

```bash
systemctl status auto-rename.service
```

## 日志查看方法

实时查看：

```bash
journalctl -u auto-rename.service -f
```

查看最近 200 行：

```bash
journalctl -u auto-rename.service -n 200 --no-pager
```

## 代码结构

- `main.go`：主程序与核心逻辑
- `go.mod`：Go 模块定义与依赖
- `auto-rename.service`：systemd 服务示例

## 备注

当前配置使用代码内默认值，结构已预留 `Config`，后续可很容易扩展为配置文件（YAML/JSON/ENV）。
