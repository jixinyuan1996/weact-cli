# lark-cli → WeAct CLI 适配改造方案

> 基于 `github.com/larksuite/cli` (v1.0.59) 源码全面分析
> 目标：将飞书官方 CLI 改造为企业私有部署版 WeAct CLI

---

## 一、项目概览

| 属性 | 值 |
|------|-----|
| 仓库 | `github.com/larksuite/cli` |
| 语言 | Go 1.23 |
| 模块路径 | `github.com/larksuite/cli` |
| npm 包名 | `@larksuite/cli` |
| 二进制名 | `lark-cli` |
| 协议 | MIT |
| 架构 | cobra 命令树 + 三层命令系统 |

### 目录结构速览

```
lark-cli/
├── main.go / main_authsidecar.go / main_noauthsidecar.go  # 入口
├── cmd/                    # cobra 命令树（root、auth、config、api、service、event、skill 等）
├── internal/               # 内部包（39 个子包）
│   ├── core/               # 配置、brand、端点、workspace
│   ├── auth/               # OAuth Device Flow、UAT、token 管理
│   ├── credential/         # 凭证 Provider 链
│   ├── client/             # APIClient（DoSDKRequest / DoStream）
│   ├── transport/          # HTTP transport 组装、代理配置
│   ├── registry/           # API 元数据加载（嵌入 + 远程）
│   ├── selfupdate/         # 自更新、Skills 同步
│   ├── cmdutil/            # Factory、IOStreams、transport decorator
│   └── ...
├── extension/              # 公共插件 SDK（credential / transport / platform）
├── shortcuts/              # 业务域快捷命令（20+ 域）
├── skills/                 # AI Agent 技能（27 个，build 时 embed 进二进制）
├── sidecar/                # Auth sidecar 协议 + demo server
├── scripts/                # npm 安装/运行脚本
├── Makefile / build.sh / .goreleaser.yml  # 构建系统
└── package.json            # npm 分发元数据
```

---

## 二、端点 URL 系统分析

### 2.1 端点解析中枢：`internal/core/types.go`

**这是所有 API 端点的唯一来源。** 所有生产代码通过 `ResolveEndpoints(brand)` 获取端点。

```go
// 当前仅支持两种 brand
type LarkBrand string

const (
    BrandFeishu LarkBrand = "feishu"   // 中国大陆（默认）
    BrandLark   LarkBrand = "lark"      // 国际版
)

func ResolveEndpoints(brand LarkBrand) Endpoints {
    switch brand {
    case BrandLark:
        return Endpoints{
            Open:     "https://open.larksuite.com",
            Accounts: "https://accounts.larksuite.com",
            MCP:      "https://mcp.larksuite.com",
            AppLink:  "https://applink.larksuite.com",
        }
    default:  // 包括 BrandFeishu 和任何未识别的字符串
        return Endpoints{
            Open:     "https://open.feishu.cn",
            Accounts: "https://accounts.feishu.cn",
            MCP:      "https://mcp.feishu.cn",
            AppLink:  "https://applink.feishu.cn",
        }
    }
}
```

**关键发现**：
- `ParseBrand()` 只识别 `"lark"`，其他任何字符串都 fallback 到 `feishu`
- 端点 URL **不能在 config.json 中单独自定义**，只能通过 brand 二选一
- `OAuthTokenV3Path = "/oauth/v3/token"` 是统一的 OAuth 2.0 端点路径

### 2.2 所有硬编码 URL 清单

| 文件:行 | URL | 用途 | 是否按 brand 切换 |
|---------|-----|------|:---:|
| `internal/core/types.go:44-54` | `open/accounts/mcp/applink.feishu.cn` / `.larksuite.com` | API 端点 | ✅ |
| `internal/registry/remote.go:81-83` | `open.feishu.cn/api/tools/open/api_definition` | 远程 API 元数据 | ✅ |
| `internal/registry/scope_hint.go:62-65` | `open.feishu.cn/page/scope-apply` | Scope 申请控制台 URL | ✅ |
| `internal/selfupdate/updater.go:51` | `open.feishu.cn/.well-known/skills/index.json` | Skills 索引 | ❌ **硬编码 feishu** |
| `internal/selfupdate/updater.go:214-241` | `open.feishu.cn` (多处) | Skills 安装/列表 | ❌ **硬编码 feishu** |
| `internal/update/update.go:26` | `registry.npmjs.org/@larksuite/cli/latest` | npm 版本检查 | N/A |
| `cmd/root.go:65` | `open.feishu.cn/document/` | 帮助文本 | ❌ 硬编码 |
| `cmd/root.go:60,63-64` | `github.com/larksuite/cli` | 帮助文本 | N/A |
| `scripts/install.js:13,42` | `registry.npmmirror.com` / `github.com/larksuite/cli/releases/` | npm 安装下载 | N/A |

---

## 三、认证系统分析

### 3.1 凭证 Provider 链

```
优先级从高到低：
1. Extension Provider（env provider）      → LARKSUITE_CLI_APP_ID/APP_SECRET/UAT/TAT
2. Extension Provider（sidecar provider）   → authsidecar build tag 时激活
3. Default Provider                         → ~/.lark-cli/config.json + keychain
```

### 3.2 OAuth 端点（`internal/auth/paths.go`）

```go
PathDeviceAuthorization   = "/oauth/v1/device_authorization"     // → {Accounts}/
PathOAuthRevoke           = "/oauth/v1/revoke"                   // → {Accounts}/
PathAppRegistration       = "/oauth/v1/app/registration"         // → {Accounts}/
PathOAuthTokenV2          = "/open-apis/authen/v2/oauth/token"   // → {Open}/
PathUserInfoV1            = "/open-apis/authen/v1/user_info"     // → {Open}/
PathApplicationInfoV6Prefix = "/open-apis/application/v6/applications/" // → {Open}/
OAuthTokenV3Path          = "/oauth/v3/token"                    // → {Accounts}/
```

### 3.3 认证流程

```
App 配置 (config init --new)
  → device flow: {Accounts}/oauth/v1/app/registration
  → 获取 AppID + AppSecret → 存 config.json + keychain

用户登录 (auth login)
  → device flow: {Accounts}/oauth/v1/device_authorization
  → 浏览器授权 → 轮询 {Open}/open-apis/authen/v2/oauth/token
  → 获取 UAT + refresh_token → 存 keychain

运行时 API 调用
  → ResolveAccount → ResolveToken（UAT 过期则 refresh）
  → APIClient.DoSDKRequest → {Open}/<path>
```

### 3.4 Auth Sidecar 模式（沙箱凭证隔离）

- 客户端：`LARKSUITE_CLI_AUTH_PROXY` 环境变量指定 sidecar 地址
- Wire 协议：HMAC-SHA256 签名 + 自定义头（`X-Lark-Proxy-*`）
- Sidecar 地址校验：必须 loopback（`127.0.0.1`/`::1`），scheme 必须 `http`
- 此模式与 brand 无关，可直接复用

---

## 四、配置文件系统分析

### 4.1 配置目录

```
优先级：
  1. LARKSUITE_CLI_CONFIG_DIR 环境变量（如果设置）
  2. ~/.lark-cli/

Workspace 隔离（子目录）：
  - 默认：~/.lark-cli/
  - OpenClaw：~/.lark-cli/openclaw/
  - Hermes：~/.lark-cli/hermes/
  - Lark Channel：~/.lark-cli/lark-channel/
```

### 4.2 主配置文件 `config.json`

```json
{
  "strictMode": "off",
  "currentApp": "my-app",
  "apps": [
    {
      "name": "my-app",
      "appId": "cli_xxx",
      "appSecret": { "source": "keychain", "id": "appsecret:cli_xxx" },
      "brand": "feishu",
      "lang": "zh",
      "defaultAs": "auto",
      "users": [{ "openId": "ou_xxx" }]
    }
  ]
}
```

**关键字段**：`brand` 字段只能是 `"feishu"` 或 `"lark"`，决定所有端点 URL。

### 4.3 环境变量（`internal/envvars/envvars.go`）

| 环境变量 | 用途 |
|----------|------|
| `LARKSUITE_CLI_APP_ID` | App ID |
| `LARKSUITE_CLI_APP_SECRET` | App Secret |
| `LARKSUITE_CLI_BRAND` | 品牌（feishu/lark） |
| `LARKSUITE_CLI_USER_ACCESS_TOKEN` | 用户访问令牌 |
| `LARKSUITE_CLI_TENANT_ACCESS_TOKEN` | 租户访问令牌 |
| `LARKSUITE_CLI_CONFIG_DIR` | 配置目录 |
| `LARKSUITE_CLI_AUTH_PROXY` | Sidecar 地址 |
| `LARKSUITE_CLI_STRICT_MODE` | 严格模式 |
| `LARKSUITE_CLI_PROXY_ENABLE/ADDRESS/CA_PATH` | 代理配置 |
| `LARKSUITE_CLI_REMOTE_META` | 远程元数据开关（=off 关闭） |

---

## 五、Skills 系统分析

### 5.1 Skills 列表（27 个）

```
lark-approval / lark-apps / lark-attendance / lark-base / lark-calendar
lark-contact / lark-doc / lark-drive / lark-event / lark-im / lark-mail
lark-markdown / lark-minutes / lark-note / lark-okr / lark-openapi-explorer
lark-shared / lark-sheets / lark-skill-maker / lark-slides / lark-task
lark-vc / lark-vc-agent / lark-whiteboard / lark-wiki
lark-workflow-meeting-summary / lark-workflow-standup-report
```

### 5.2 Embed 机制

- `skills_embed.go` 通过 `//go:embed skills/*/SKILL.md skills/*/references skills/*/routes skills/*/scenes` 嵌入
- 白名单模式：新增子目录类型（如 `assets/`）会被静默忽略
- 总大小约 3.3 MB

### 5.3 Skills 更新机制

- 索引 URL：`https://open.feishu.cn/.well-known/skills/index.json`（**硬编码**）
- 安装：`npx skills add larksuite/cli -s <skill-name>`
- 自更新时同步 skills

---

## 六、详细改动方案

### 6.1 P0 — 必须改动（不改无法连接 WeAct）

#### 6.1.1 `internal/core/types.go` — 端点定义中枢

**改动内容**：
1. 新增 `BrandWeAct LarkBrand = "weact"` 常量
2. `ParseBrand()` 增加 `"weact"` 识别
3. `ResolveEndpoints()` 新增 weact case，从环境变量读取端点（支持自定义域名）

**建议实现方式**：通过环境变量覆盖，保持灵活性

```go
// 新增常量
const BrandWeAct LarkBrand = "weact"

// 改造 ParseBrand
func ParseBrand(value string) LarkBrand {
    switch value {
    case "lark":
        return BrandLark
    case "weact":
        return BrandWeAct
    default:
        return BrandFeishu
    }
}

// 改造 ResolveEndpoints
func ResolveEndpoints(brand LarkBrand) Endpoints {
    switch brand {
    case BrandLark:
        return Endpoints{
            Open:     "https://open.larksuite.com",
            Accounts: "https://accounts.larksuite.com",
            MCP:      "https://mcp.larksuite.com",
            AppLink:  "https://applink.larksuite.com",
        }
    case BrandWeAct:
        return Endpoints{
            Open:     getEnvOrDefault("WEACT_OPEN_ENDPOINT", "https://open.weact.cn"),
            Accounts: getEnvOrDefault("WEACT_ACCOUNTS_ENDPOINT", "https://accounts.weact.cn"),
            MCP:      getEnvOrDefault("WEACT_MCP_ENDPOINT", "https://mcp.weact.cn"),
            AppLink:  getEnvOrDefault("WEACT_APPLINK_ENDPOINT", "https://applink.weact.cn"),
        }
    default:
        return Endpoints{
            Open:     "https://open.feishu.cn",
            Accounts: "https://accounts.feishu.cn",
            MCP:      "https://mcp.feishu.cn",
            AppLink:  "https://applink.feishu.cn",
        }
    }
}
```

**影响范围**：所有调用 `core.ResolveEndpoints()` 的代码自动跟随，无需逐文件修改。包括：
- `internal/auth/device_flow.go` — OAuth Device Flow
- `internal/auth/uat_client.go` — UAT 刷新
- `internal/auth/app_registration.go` — App 注册
- `internal/auth/revoke.go` — Token 撤销
- `internal/credential/tat_fetch.go` — TAT 获取
- `internal/credential/user_info.go` — 用户信息
- `internal/client/client.go` — API 调用
- `internal/cmdutil/factory_default.go` — Client 工厂
- `sidecar/server-demo/` — Sidecar demo

#### 6.1.2 `internal/registry/remote.go` — 远程 API 元数据

**改动内容**：`remoteMetaURL()` 新增 weact case

```go
func remoteMetaURL(version string) string {
    // ...
    var base string
    switch configuredBrand {
    case core.BrandLark:
        base = "https://open.larksuite.com/api/tools/open/api_definition"
    case core.BrandWeAct:
        base = getEnvOrDefault("WEACT_API_DEFINITION_URL",
            "https://open.weact.cn/api/tools/open/api_definition")
    default:
        base = "https://open.feishu.cn/api/tools/open/api_definition"
    }
    // ...
}
```

#### 6.1.3 `internal/registry/scope_hint.go` — 控制台 Scope URL

**改动内容**：`BuildConsoleScopeURL()` 新增 weact case

```go
func BuildConsoleScopeURL(brand core.LarkBrand, appID, scope string) string {
    // ...
    host := "open.feishu.cn"
    switch brand {
    case core.BrandLark:
        host = "open.larksuite.com"
    case core.BrandWeAct:
        host = getEnvOrDefault("WEACT_CONSOLE_HOST", "open.weact.cn")
    }
    // ...
}
```

#### 6.1.4 `internal/selfupdate/updater.go` — Skills 索引 URL

**现状**：`officialSkillsIndexURL` 硬编码 `https://open.feishu.cn/.well-known/skills/index.json`

**改动内容**：改为变量，按 brand 或环境变量切换

```go
// 将硬编码常量改为函数
func getSkillsIndexURL() string {
    if url := os.Getenv("WEACT_SKILLS_INDEX_URL"); url != "" {
        return url
    }
    // 根据 brand 返回不同 URL
    // ...
    return "https://open.feishu.cn/.well-known/skills/index.json"
}
```

#### 6.1.5 `internal/core/workspace.go` — 配置目录

**改动内容**：`GetBaseConfigDir()` 中 fallback 路径从 `.lark-cli` 改为 `.weact-cli`

```go
func GetBaseConfigDir() string {
    if dir := os.Getenv("LARKSUITE_CLI_CONFIG_DIR"); dir != "" {
        return dir
    }
    // 或者新增 WEACT_CLI_CONFIG_DIR 支持
    if dir := os.Getenv("WEACT_CLI_CONFIG_DIR"); dir != "" {
        return dir
    }
    home, _ := vfs.UserHomeDir()
    return filepath.Join(home, ".weact-cli")  // 原为 .lark-cli
}
```

---

### 6.2 P1 — 品牌体验一致性

#### 6.2.1 `cmd/root.go` — CLI 名称和帮助文本

| 位置 | 当前内容 | 改为 |
|------|---------|------|
| `rootLong` 首行 | `lark-cli — Lark/Feishu CLI tool.` | `weact-cli — WeAct CLI tool.` |
| `rootLong` 示例 | `lark-cli <command>` | `weact-cli <command>` |
| `rootLong` 文档链接 | `https://open.feishu.cn/document/` | WeAct 内部文档地址 |
| `rootLong` GitHub | `https://github.com/larksuite/cli` | WeAct 内部仓库地址 |
| Skills 提示 | `npx skills add larksuite/cli` | `npx skills add weact/cli` |

**建议**：通过 `-ldflags` 注入可变的名称和链接，减少硬编码

#### 6.2.2 `internal/build/` — 版本信息

**改动内容**：新增 `AppName` 变量，通过 `-ldflags` 注入

```go
var AppName = "weact-cli"  // 通过 -ldflags 覆盖
```

#### 6.2.3 `Makefile` / `build.sh`

**改动内容**：
- 二进制名 `lark-cli` → `weact-cli`
- 新增 `WEACT_VERSION` 等构建变量
- 添加 `weact` build tag 支持

```makefile
BINARY_NAME ?= weact-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(MODULE)/internal/build.Version=$(VERSION) -X $(MODULE)/internal/build.Date=$(DATE) -X $(MODULE)/internal/build.AppName=$(BINARY_NAME)

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .
```

#### 6.2.4 `package.json`

```json
{
  "name": "@weact/cli",
  "version": "1.0.0",
  "description": "The official CLI for WeAct open platform",
  "bin": { "weact-cli": "scripts/run.js" },
  "repository": { "type": "git", "url": "git+https://git.weact.cn/cli.git" }
}
```

#### 6.2.5 `scripts/install.js`

**改动内容**：
- GitHub Release URL → WeAct 私有制品仓库
- `ALLOWED_HOSTS` 更新为 WeAct 域名
- npm registry 改为私有 registry

#### 6.2.6 `scripts/run.js`

**改动内容**：二进制名称 `lark-cli` → `weact-cli`

#### 6.2.7 `skills/**/SKILL.md` — 品牌名批量替换

**批量搜索替换规则**：

| 搜索 | 替换 |
|------|------|
| `Lark/Feishu` | `WeAct` |
| `Lark`（品牌语境） | `WeAct` |
| `larksuite` | `weact` |
| `open.feishu.cn` | `open.weact.cn` |
| `open.larksuite.com` | `open.weact.cn` |
| `feishu.cn` | `weact.cn` |
| `lark-cli` | `weact-cli` |

**注意**：`lark` 作为 API 包名/技术术语（如 `larkws`、`lark_channel`）时**不要替换**。

---

### 6.3 P2 — 可后续迭代

#### 6.3.1 环境变量前缀

考虑是否新增 `WEACT_CLI_*` 系列环境变量，同时兼容 `LARKSUITE_CLI_*`：

```go
// 同时检查两个前缀
func getWeactEnv(key string) string {
    if v := os.Getenv("WEACT_CLI_" + key); v != "" {
        return v
    }
    return os.Getenv("LARKSUITE_CLI_" + key)
}
```

#### 6.3.2 Shortcuts 中的域名注释

以下文件中的注释 URL 建议替换：

| 文件 | 行 | 内容 |
|------|-----|------|
| `shortcuts/vc/vc_recording.go` | 42 | `meetings.feishu.cn/minutes/` |
| `shortcuts/wiki/wiki_node_get.go` | 72 | `feishu.cn/wiki/` |
| `shortcuts/wiki/wiki_node_delete.go` | 68 | `feishu.cn/docx/` |

#### 6.3.3 端到端测试

`tests/cli_e2e/` 中的测试用例需要适配 WeAct 端点。

---

## 七、改动文件汇总

### 按优先级排序

| 优先级 | 文件 | 改动类型 | 改动量 |
|:---:|------|------|:---:|
| **P0** | `internal/core/types.go` | 新增 BrandWeAct + 端点解析 | 中 |
| **P0** | `internal/registry/remote.go` | 新增 weact 元数据 URL | 小 |
| **P0** | `internal/registry/scope_hint.go` | 新增 weact 控制台 URL | 小 |
| **P0** | `internal/selfupdate/updater.go` | Skills 索引 URL 可配置 | 中 |
| **P0** | `internal/core/workspace.go` | 配置目录改为 `.weact-cli` | 小 |
| **P1** | `cmd/root.go` | CLI 名称、帮助文本 | 中 |
| **P1** | `Makefile` / `build.sh` | 二进制名、构建变量 | 小 |
| **P1** | `.goreleaser.yml` | 发布目标 | 小 |
| **P1** | `package.json` | npm 包元数据 | 小 |
| **P1** | `scripts/install.js` | 下载来源 | 中 |
| **P1** | `scripts/run.js` | 二进制名 | 小 |
| **P1** | `skills/**/SKILL.md` | 品牌名/域名替换 | **大** |
| **P1** | `skill-template/**` | 模板替换 | 小 |
| **P1** | `internal/envvars/envvars.go` | 新增 WeAct 环境变量 | 小 |
| **P2** | `shortcuts/**/*.go` | 注释中的域名 | 小 |
| **P2** | `internal/update/update.go` | npm registry URL | 小 |
| **P2** | `tests/cli_e2e/` | 端到端测试适配 | 中 |
| **P2** | `internal/keychain/keychain.go` | 服务名 `lark-cli` | 小 |
| **P2** | `README.md` / `README.zh.md` | 文档更新 | 中 |
| **P2** | `AGENTS.md` | 开发文档更新 | 中 |

---

## 八、不改动的部分

以下部分**无需改动**，可直接复用：

- **认证路径**（`internal/auth/paths.go`）：OAuth 2.0 标准路径，私有部署版应兼容
- **API 路径结构**：`/open-apis/<service>/<version>/<resource>` 为标准飞书 OpenAPI 格式
- **凭证 Provider 链**（`internal/credential/`）：机制通用，与 brand 无关
- **Auth Sidecar**（`sidecar/`）：沙箱凭证隔离机制完全通用
- **命令系统**（`cmd/service/`、`shortcuts/`）：命令注册和执行管线与 brand 无关
- **输出格式化**（`internal/output/`）：JSON/NDJSON/Table/CSV 通用
- **事件系统**（`internal/event/`）：WebSocket 事件源可通过 brand 切换端点
- **Keychain**（`internal/keychain/`）：跨平台安全存储机制通用
- **Transport 代理**（`internal/transport/`）：代理配置机制通用
- **Policy 引擎**（`internal/cmdpolicy/`）：用户层策略通用
- **错误分类**（`internal/errclass/`、`errs/`）：错误类型系统通用

---

## 九、实施建议

### 9.1 推荐策略：环境变量覆盖 + 新增 brand

**核心思路**：
1. 在 `ResolveEndpoints()` 中新增 `BrandWeAct`，端点 URL 通过环境变量读取，带默认值
2. 同时保留 `LARKSUITE_CLI_*` 环境变量兼容性
3. Skills 和自更新 URL 也通过环境变量覆盖
4. 通过 `-ldflags` 注入 CLI 名称和版本信息

**优点**：
- 改动集中在 5-6 个核心文件
- 不破坏与上游的兼容性（feishu/lark 仍可用）
- 私有部署用户只需设置环境变量即可切换端点
- 后续上游更新时 merge 冲突最小

### 9.2 使用方式

```bash
# 设置 WeAct 品牌
export LARKSUITE_CLI_BRAND=weact

# 设置自定义端点（可选，不设置则用默认值）
export WEACT_OPEN_ENDPOINT=https://open.weact.example.com
export WEACT_ACCOUNTS_ENDPOINT=https://accounts.weact.example.com
export WEACT_MCP_ENDPOINT=https://mcp.weact.example.com
export WEACT_APPLINK_ENDPOINT=https://applink.weact.example.com

# 配置 App
weact-cli config init --new --brand weact

# 登录
weact-cli auth login

# 使用
weact-cli calendar +agenda
```

### 9.3 构建命令

```bash
# 构建 WeAct 版本
make build BINARY_NAME=weact-cli

# 或通过 ldflags 注入
go build -ldflags "-X github.com/larksuite/cli/internal/build.AppName=weact-cli" -o weact-cli .
```

---

## 十、附录：关键文件路径速查

### 核心文件（优先阅读）

| 文件 | 用途 |
|------|------|
| `internal/core/types.go` | **端点定义中枢** |
| `internal/core/workspace.go` | 配置目录、Workspace 隔离 |
| `internal/core/config.go` | 多 App 配置加载/保存 |
| `internal/auth/device_flow.go` | OAuth Device Flow |
| `internal/auth/uat_client.go` | UAT 刷新/撤销/验证 |
| `internal/credential/tat_fetch.go` | TAT 获取 |
| `internal/credential/credential_provider.go` | 凭证 Provider 链入口 |
| `internal/registry/remote.go` | 远程 API 元数据获取 |
| `internal/registry/scope_hint.go` | 控制台 Scope URL |
| `internal/selfupdate/updater.go` | Skills 索引 + 自更新 |
| `internal/update/update.go` | npm 版本检查 |
| `cmd/root.go` | CLI 入口、帮助文本 |
| `internal/envvars/envvars.go` | 环境变量常量 |
| `internal/build/` | 版本/日期变量（ldflags 注入） |

### 构建相关

| 文件 | 用途 |
|------|------|
| `Makefile` | 主构建脚本 |
| `build.sh` | 便捷构建脚本 |
| `.goreleaser.yml` | GoReleaser 发布配置 |
| `package.json` | npm 包元数据 |
| `scripts/install.js` | npm postinstall 下载二进制 |
| `scripts/run.js` | npm bin 入口 |
| `skills_embed.go` | Skills embed 指令 |

### Skills

| 路径 | 用途 |
|------|------|
| `skills/*/SKILL.md` | 各域 AI Agent 技能主文档 |
| `skills/lark-shared/SKILL.md` | 共享规则（认证/权限/身份） |
| `skill-template/` | 新建 skill 模板 |
