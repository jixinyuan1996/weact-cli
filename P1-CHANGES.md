# P1 改动完成记录

> 基于 `github.com/larksuite/cli` (v1.0.59) 品牌体验一致性改造
> 目标：将所有用户可见的 CLI 名称、帮助文本、品牌名从 lark-cli / Lark/Feishu 改为 weact-cli / WeAct

---

## 改动的文件清单

### 核心品牌标识

| 文件 | 改动内容 |
|------|----------|
| `internal/build/build.go` | 新增 `AppName = "weact-cli"` 变量，可通过 `-ldflags` 覆盖 |
| `cmd/build.go` | cobra 根命令 `Use` 字段从硬编码 `"lark-cli"` 改为 `build.AppName`，`Short` 改为 WeAct CLI |
| `cmd/root.go` | `rootLong` 帮助文本全部改为 `weact-cli` / `WeAct`；`composePendingNotice` 中的 `"command"` 字段改为 `"weact-cli update"` |

### 构建系统

| 文件 | 改动内容 |
|------|----------|
| `Makefile` | `BINARY := lark-cli` → `BINARY := weact-cli`；`LDFLAGS` 新增 `-X .../build.AppName=$(BINARY)` |

### npm 包

| 文件 | 改动内容 |
|------|----------|
| `package.json` | `name`: `@larksuite/cli` → `@weact/cli`；`version` 重置为 `1.0.0`；`description` 改为 WeAct；`bin` 入口 key 改为 `weact-cli`；`repository.url` 改为 WeAct 仓库 |
| `scripts/run.js` | 二进制引用路径 `lark-cli` → `weact-cli`（含错误信息） |
| `scripts/install.js` | `REPO` / `NAME` 改为 WeAct；npm 镜像路径 `/-/binary/lark-cli/` → `/-/binary/weact-cli/`；错误提示中包名改为 `@weact/cli` |

### 环境变量

| 文件 | 改动内容 |
|------|----------|
| `internal/envvars/envvars.go` | 新增 WeAct 系列环境变量常量（`WeActOpenEndpoint`、`WeActAccountsEndpoint`、`WeActMCPEndpoint`、`WeActAppLinkEndpoint`、`WeActAPIDefinitionURL`、`WeActConsoleHost`、`WeActSkillsIndexURL`、`WeActSkillsSource`、`WeActConfigDir`） |

### cmd/ 子包（用户可见字符串）

以下文件中所有用户可见的 `lark-cli` 已改为 `weact-cli`，`~/.lark-cli` 改为 `~/.weact-cli`：

- `cmd/auth/check.go`、`cmd/auth/list.go`、`cmd/auth/login.go`、`cmd/auth/login_messages.go`、`cmd/auth/status.go`
- `cmd/config/bind.go`、`cmd/config/bind_messages.go`、`cmd/config/init.go`、`cmd/config/init_messages.go`、`cmd/config/init_probe.go`、`cmd/config/strict_mode.go`、`cmd/config/show.go`、`cmd/config/plugins.go`、`cmd/config/keychain_downgrade.go`
- `cmd/doctor/doctor.go`
- `cmd/event/consume.go`、`cmd/event/schema.go`、`cmd/event/status.go`、`cmd/event/suggestions.go`
- `cmd/profile/remove.go`、`cmd/profile/use.go`
- `cmd/prune.go`、`cmd/platform_bootstrap.go`、`cmd/platform_guards.go`
- `cmd/service/affordance.go`、`cmd/service/paramhelp.go`、`cmd/service/service.go`
- `cmd/skill/skill.go`
- `cmd/update/update.go`

### Skills 文档（301 个文件批量替换）

对 `skills/` 和 `skill-template/` 下所有文件执行以下替换（保留 `larkws`、`lark_channel` 等技术术语不变）：

| 搜索 | 替换 |
|------|------|
| `Lark/Feishu` | `WeAct` |
| `lark-cli` | `weact-cli` |
| `larksuite/cli` | `weact/cli` |
| `open.feishu.cn` | `open.weact.cn` |
| `open.larksuite.com` | `open.weact.cn` |
| `feishu.cn` | `weact.cn` |

---

## 改动原则

- **ldflags 驱动**：`AppName` 通过构建参数注入，cobra `Use` 字段动态读取，保证二进制名与帮助文本始终一致
- **兼容上游**：未修改任何 Go 模块路径（`github.com/larksuite/cli`）和内部包名，上游更新可低冲突 merge
- **技术术语保留**：`larkws`、`lark_channel`、`lark-channel` 等 API 技术名词未做替换

---

## 构建命令

```bash
# 直接构建（AppName 通过 ldflags 注入）
go build -trimpath \
  -ldflags "-s -w \
    -X github.com/larksuite/cli/internal/build.Version=1.0.0 \
    -X github.com/larksuite/cli/internal/build.Date=$(date +%Y-%m-%d) \
    -X github.com/larksuite/cli/internal/build.AppName=weact-cli" \
  -o weact-cli .

# 或使用 Makefile
make build
```

---

## 验证结果

```
$ ./weact-cli --version
weact-cli version 1.0.0

$ ./weact-cli --help
weact-cli — WeAct CLI tool.

USAGE:
    weact-cli <command> [subcommand] [method] [options]
    ...

Use "weact-cli [command] --help" for more information about a command.
```

---

## 后续工作（P2）

- [ ] 环境变量前缀统一：`WEACT_CLI_*` 同时兼容 `LARKSUITE_CLI_*`
- [ ] `shortcuts/` 中注释里的域名（非用户可见，低优先级）
- [ ] `internal/keychain/keychain.go` keychain 服务名 `lark-cli`（影响 macOS Keychain 条目名称）
- [ ] `internal/update/update.go` npm registry 检查 URL 指向私有 registry
- [ ] E2E 测试适配 WeAct 端点
- [ ] `.goreleaser.yml` 发布配置更新
