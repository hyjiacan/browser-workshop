# Serve 自动同步

bws sv 支持自动从在线源同步浏览器安装包到本地 `packages/` 目录，实现离线源的自动更新。

## 功能说明

自动同步是 serve 服务的可选功能，启用后会定期从在线源下载最新的浏览器安装包到本地，使离线源始终保持最新。

### 主要特性

- **定时同步**：按照指定的间隔自动执行同步
- **增量下载**：已存在的文件跳过，只下载新文件
- **多浏览器支持**：可配置同步多个浏览器
- **多渠道支持**：可配置同步多个发布渠道
- **手动触发**：支持通过 Web 页面或 API 手动触发同步
- **状态查询**：可查询当前同步状态和历史记录
- **自动刷新**：同步完成后自动刷新文件清单

### 同步流程

1. 从在线源获取可用版本列表
2. 与本地 `packages/` 目录对比，找出缺失的版本
3. 逐个下载缺失的安装包
4. 下载完成后保存到 `packages/` 目录
5. 自动刷新文件清单和校验和缓存

## 启用同步

编辑 `bws-serve.ini` 配置文件，启用自动同步功能。

### 基本配置

```ini
[serve]
sync = true
sync-interval = 24h
sync-channels = stable
```

然后运行 `bws sv` 启动服务。

默认配置：
- 同步间隔：24 小时（每天一次）
- 同步浏览器：所有支持的浏览器
- 同步渠道：stable

### 启动时立即同步

启用同步后，服务启动时会立即执行一次同步，之后按照间隔定时执行。

## 同步间隔配置

使用 `sync-interval` 配置项自定义同步间隔。

### 时间格式

支持以下时间单位：

| 单位 | 缩写 | 示例 |
|------|------|------|
| 天 | `d` | `7d`, `30d` |
| 小时 | `h` | `6h`, `12h`, `24h` |
| 分钟 | `m` | `30m`, `60m` |
| 秒 | `s` | `3600s` |

也可以组合使用：`1h30m`、`2h45m` 等。

### 常用配置

```ini
# 每 30 天同步一次
sync-interval = 30d

# 每 6 小时同步一次
sync-interval = 6h

# 每 12 小时同步一次
sync-interval = 12h

# 每天同步一次（默认）
sync-interval = 24h
```

### 间隔选择建议

- **开发环境**：`6h` 或 `12h` - 保持较新版本
- **生产环境**：`24h` - 每天更新一次即可
- **测试环境**：`1h` - 需要频繁获取最新版本时使用

> **注意**：同步间隔不宜过短，否则会给在线源造成压力。建议最短间隔不低于 1 小时。

## 同步的浏览器和渠道

默认情况下，同步所有支持的浏览器的 stable 渠道版本。可以通过配置项自定义。

### 指定同步的浏览器

使用 `sync-browsers` 配置项指定要同步的浏览器列表，多个浏览器用逗号分隔：

```ini
# 只同步 Chrome
sync-browsers = chrome

# 同步 Chrome 和 Firefox
sync-browsers = chrome,firefox

# 同步 Chrome、Firefox、Chromium
sync-browsers = chrome,firefox,chromium
```

支持的浏览器名称：
- `chrome` - Google Chrome
- `firefox` - Mozilla Firefox
- `chromium` - Chromium
- `edge` - Microsoft Edge

也可以使用短别名：
- `gc` = chrome
- `ff` = firefox
- `cm` = chromium

### 指定同步的渠道

使用 `sync-channels` 配置项指定要同步的渠道列表，多个渠道用逗号分隔：

```ini
# 只同步 stable 渠道（默认）
sync-channels = stable

# 同步 stable 和 beta 渠道
sync-channels = stable,beta

# 同步所有渠道
sync-channels = stable,beta,dev,canary
```

支持的渠道：
- `stable` - 稳定版
- `beta` - Beta 测试版
- `dev` - Dev 开发版
- `canary` - Canary 金丝雀版
- `esr` - Firefox 延长支持版

### 组合配置示例

```ini
# 同步 Chrome 的 stable 和 beta 渠道，每 12 小时一次
[serve]
sync = true
sync-interval = 12h
sync-browsers = chrome
sync-channels = stable,beta
```

```ini
# 同步 Chrome 和 Firefox 的 stable 渠道，每天一次
[serve]
sync = true
sync-browsers = chrome,firefox
sync-channels = stable
```

配置完成后，运行 `bws sv` 启动服务。

## Web 页面手动触发

除了定时自动同步，还可以通过 Web 页面手动触发同步。

### 操作步骤

1. 在浏览器中打开 serve 服务地址（如 `http://server:8080`）
2. 找到"同步"或"立即同步"按钮
3. 点击按钮触发同步
4. 页面会显示同步进度和结果

### 使用场景

- 刚发布新版本，不想等到下一次定时同步
- 测试同步功能是否正常工作
- 临时需要某个新版本

手动触发的同步与定时同步的流程完全相同，不会影响定时同步的调度。

## 同步状态 API

可以通过 API 查询同步状态。

### 查询同步状态

**请求：**

```
GET /api/v1/sync/status
```

**响应示例：**

```json
{
  "running": false,
  "lastSync": "2024-01-15T10:30:00Z",
  "nextSync": "2024-01-16T10:30:00Z",
  "lastResult": {
    "success": 5,
    "failed": 0,
    "skipped": 10,
    "total": 15
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `running` | boolean | 当前是否正在同步 |
| `lastSync` | string | 上次同步时间（ISO 8601 格式） |
| `nextSync` | string | 下次计划同步时间（ISO 8601 格式） |
| `lastResult.success` | number | 上次同步成功下载的文件数 |
| `lastResult.failed` | number | 上次同步失败的文件数 |
| `lastResult.skipped` | number | 上次同步跳过的文件数（已存在） |
| `lastResult.total` | number | 上次同步处理的总文件数 |

### 手动触发同步 API

**请求：**

```
POST /api/v1/sync/trigger
```

**响应示例：**

```json
{
  "ok": true,
  "message": "同步已触发"
}
```

如果同步正在进行中，会返回错误：

```json
{
  "ok": false,
  "error": "同步正在进行中，请稍候再试"
}
```

更多 API 详细信息请参考 [Serve API 参考](./serve-api.md) 章节。

## 注意事项

1. **磁盘空间**：同步的文件可能占用大量磁盘空间，尤其是同步多个浏览器和渠道时
2. **网络带宽**：同步会下载大量文件，注意网络带宽占用
3. **同步冲突**：手动触发同步时，如果定时同步正在进行，会被拒绝
4. **文件校验**：下载完成后会自动校验文件完整性，失败的文件会被重新下载
5. **增量同步**：已存在的文件会被跳过，不会重复下载
6. **服务重启**：服务重启后，同步调度会重新计算，可能导致同步时间偏移