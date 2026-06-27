# P0 改动完成记录

> 基于 `github.com/larksuite/cli` (v1.0.59) 适配 WeAct 企业私有部署版飞书 CLI

## 改动的 5 个文件

| 文件 | 改动内容 |
|------|----------|
| `internal/core/types.go` | 新增 `BrandWeAct` 常量、`ParseBrand` 识别 `"weact"`、`ResolveEndpoints` 新增 WeAct case（从环境变量读端点）、导出 `GetenvOrDefault` 辅助函数 |
| `internal/core/workspace.go` | 配置目录 fallback 从 `.lark-cli` → `.weact-cli`，新增 `WEACT_CLI_CONFIG_DIR` 优先级 |
| `internal/registry/remote.go` | `remoteMetaURL()` 新增 `BrandWeAct` case，从 `WEACT_API_DEFINITION_URL` 读取 |
| `internal/registry/scope_hint.go` | `BuildConsoleScopeURL()` 新增 `BrandWeAct` case，从 `WEACT_CONSOLE_HOST` 读取 |
| `internal/selfupdate/updater.go` | Skills 索引 URL 和安装来源从硬编码改为函数，支持 `WEACT_SKILLS_INDEX_URL` / `WEACT_SKILLS_SOURCE` 覆盖 |

## 改动原则

- **环境变量驱动**：所有 WeAct 端点均通过环境变量配置，带 `example.com` 占位默认值
- **兼容上游**：`feishu` / `lark` 两种 brand 行为不变，不影响原有功能
- **最小侵入**：改动集中在端点解析中枢，上层调用代码自动跟随，无需逐文件修改

## 使用方式

拿到 WeAct 真实域名后，设置以下环境变量：

```bash
export LARKSUITE_CLI_BRAND=weact

# 替换为 WeAct 真实域名
export WEACT_OPEN_ENDPOINT=https://open.weact.example.com
export WEACT_ACCOUNTS_ENDPOINT=https://accounts.weact.example.com
export WEACT_MCP_ENDPOINT=https://mcp.weact.example.com
export WEACT_APPLINK_ENDPOINT=https://applink.weact.example.com

# 可选：自定义其他端点
export WEACT_API_DEFINITION_URL=https://open.weact.example.com/api/tools/open/api_definition
export WEACT_CONSOLE_HOST=open.weact.example.com
export WEACT_SKILLS_INDEX_URL=https://open.weact.example.com/.well-known/skills/index.json
export WEACT_SKILLS_SOURCE=https://open.weact.example.com
export WEACT_CLI_CONFIG_DIR=/path/to/config   # 自定义配置目录
```

然后正常使用：

```bash
# 配置 App
weact-cli config init --new --brand weact

# 登录
weact-cli auth login

# 日常使用
weact-cli calendar +agenda
weact-cli im +send-message --to ou_xxx --text "Hello"
```

## 环境变量完整列表

| 环境变量 | 用途 | 默认值 |
|----------|------|--------|
| `LARKSUITE_CLI_BRAND` | 品牌切换（设为 `weact`） | `feishu` |
| `WEACT_OPEN_ENDPOINT` | Open API 地址 | `https://open.weact.example.com` |
| `WEACT_ACCOUNTS_ENDPOINT` | 账号认证服务地址 | `https://accounts.weact.example.com` |
| `WEACT_MCP_ENDPOINT` | MCP 服务地址 | `https://mcp.weact.example.com` |
| `WEACT_APPLINK_ENDPOINT` | AppLink 服务地址 | `https://applink.weact.example.com` |
| `WEACT_API_DEFINITION_URL` | API 元数据接口 | `https://open.weact.example.com/api/tools/open/api_definition` |
| `WEACT_CONSOLE_HOST` | 开发者后台域名 | `open.weact.example.com` |
| `WEACT_SKILLS_INDEX_URL` | Skills 索引地址 | 沿用飞书官方地址 |
| `WEACT_SKILLS_SOURCE` | Skills 安装来源 | `https://open.feishu.cn` |
| `WEACT_CLI_CONFIG_DIR` | 配置目录（优先级最高） | `~/.weact-cli` |
| `LARKSUITE_CLI_CONFIG_DIR` | 配置目录（兼容旧版） | `~/.weact-cli` |

## 后续工作（P1/P2）

- [ ] CLI 名称从 `lark-cli` 改为 `weact-cli`（`cmd/root.go`、`Makefile`、`package.json` 等）
- [ ] Skills 文档中品牌名/域名批量替换
- [ ] 构建脚本适配
- [ ] npm 包发布为 `@weact/cli`
- [ ] 端到端测试适配
