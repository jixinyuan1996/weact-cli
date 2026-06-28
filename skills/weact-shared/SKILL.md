---
name: weact-shared
version: 1.0.0
description: "Use when first setting up weact-cli, running auth login, switching user/bot identity (--as), handling permission denied or scope errors, needing to update weact-cli, or seeing _notice in JSON output."
---

# weact-cli 共享规则

本技能指导你如何通过weact-cli操作WeAct资源, 以及有哪些注意事项。

## 配置初始化

首次使用需运行 `weact-cli config init` 完成应用配置。

当你帮用户初始化配置时，使用background方式使用下面的命令发起配置应用流程，启动后读取输出，从中提取授权链接并发给用户。

**URL 转发规则**：当命令输出 `verification_url`、`verification_uri_complete`、`console_url` 等 URL 字段时：**必须生成二维码**：你必须调用 `weact-cli auth qrcode` 将 URL 转为二维码并展示给用户，这是必须步骤，不要跳过。优先生成 PNG 二维码（--output）；仅当用户明确要求时才使用 ASCII（--ascii）。**URL 输出规则**：将 URL 视为不可修改的 opaque string，不要做任何修改（包括 URL 编码/解码、添加空格或标点、重新拼接 query），二维码和链接请一起展示给用户。

```bash
# 发起配置（该命令会阻塞直到用户打开链接并完成操作或过期）
weact-cli config init --new
```

## 认证

### 身份类型

两种身份类型，通过 `--as` 切换：

| 身份 | 标识 | 获取方式 | 适用场景 |
|------|------|---------|---------|
| user 用户身份 | `--as user` | `weact-cli auth login` 等 | 访问用户自己的资源（日历、云空间/云盘/云存储等） |
| bot 应用身份 | `--as bot` | 自动，只需 appId + appSecret | 应用级操作,访问bot自己的资源 |

### 身份选择原则

输出的 `[identity: bot/user]` 代表当前身份。bot 与 user 表现差异很大，需确认身份符合目标需求：

- **Bot 看不到用户资源**：无法访问用户的日历、云空间（云盘/云存储）文档、邮箱等个人资源。例如 `--as bot` 查日程返回 bot 自己的（空）日历
- **Bot 无法代表用户操作**：发消息以应用名义发送，创建文档归属 bot
- **Bot 权限**：只需在WeAct开发者后台开通 scope，无需 `auth login`
- **User 权限**：后台开通 scope + 用户通过 `auth login` 授权，两层都要满足


### 权限不足处理

遇到权限相关错误时，**根据当前身份类型采取不同解决方案**。

错误响应中包含关键信息：
- `permission_violations`：列出缺失的 scope (N选1)
- `console_url`：WeAct开发者后台的权限配置链接
- `hint`：建议的修复命令

#### Bot 身份（`--as bot`）

将错误中的 `console_url` 原样提供给用户，引导去后台开通 scope。**禁止**对 bot 执行 `auth login`。

#### User 身份（`--as user`）

```bash
weact-cli auth login --domain <domain>           # 按业务域授权
weact-cli auth login --scope "<missing_scope>"   # 按具体 scope 授权（推荐,符合最小权限原则）
```

**规则**：auth login 必须指定范围（`--domain` 或 `--scope`）。多次 login 的 scope 会累积（增量授权）。

#### Agent 代理发起认证（WeAct IM 场景，推荐）

当你作为 AI agent 在 **WeAct IM 对话**中检测到 `missing_scopes` 错误时，使用以下全自动授权流程——**用户只需点一次按钮，无需手动回复**：

**第一步：发起授权**

```bash
weact-cli auth login --scope "<missing_scopes 用空格拼接>" --no-wait --json
```

从 JSON 输出中提取 `verification_url` 和 `device_code`。

**第二步：发送授权卡片**

用 `weact-cli im +messages-send` 向用户发送交互卡片（卡片 JSON 模板见 [`referenc../weact-im-auth-card.md`](referenc../weact-im-auth-card.md)，替换 `{missing_scopes}` 和 `{verification_url}` 两个占位符后发送）：

```bash
weact-cli im +messages-send \
  --user-id <用户 open_id> \
  --msg-type interactive \
  --content '<卡片 JSON>' \
  --as bot
```

卡片包含一个 `url` 类型按钮，用户点击后在 **WeAct 内置浏览器**中打开授权页，已登录态直接点"同意"即可，全程不离开 WeAct。

**第三步：后台轮询（无需等待用户回复）**

以 **background** 方式执行以下命令，不阻塞当前对话：

```bash
weact-cli auth login --device-code <device_code>
```

此命令每 5 秒轮询一次 WeAct 授权服务器，用户点击"同意"后立即返回成功。

**第四步：自动继续**

轮询返回成功后，主动向用户发消息：

> ✅ 授权完成，继续为您处理...

然后**自动重试**原始请求，将结果返回给用户。

**关键规则**：

- **`--device-code` 必须在 background 中运行**，不能阻塞当前对话轮次
- 轮询超时（默认 5 分钟）后主动告知用户"授权超时，请重新发起请求"
- **禁止缓存 `device_code`**：每次需要授权都重新执行 `--no-wait --json` 生成新的
- 卡片的 `--user-id` 取自当前对话用户的 `open_id`，从 `weact-cli auth status --json` 的 `identities.user.openId` 字段获取

#### Agent 代理发起认证（非 WeAct IM 场景）

当不在 WeAct IM 对话中（如终端、其他渠道）时，使用传统 split-flow：

1. 执行 `weact-cli auth login --scope "xxx" --no-wait --json`，提取 `verification_url` 和 `device_code`
2. 将 URL 展示给用户，明确告知"请完成授权后告诉我"
3. 等待用户回复"已完成"后，执行 `weact-cli auth login --device-code <device_code>`

**禁止缓存 `verification_url` 或 `device_code`**：每次需要授权时必须重新生成。

## 更新检查

weact-cli 命令执行后，如果检测到新版本，JSON 输出中会包含 `_notice.update` 字段（含 `message`、`command` 等）。

**当你在输出中看到 `_notice.update` 时，完成用户当前请求后，主动提议帮用户更新**：

1. 告知用户当前版本和最新版本号
2. 提议执行更新（同时更新 CLI 和 Skills）：
   ```bash
   weact-cli update
   ```
3. 更新完成后提醒用户：**退出并重新打开 AI Agent** 以加载最新 Skills

**重要**：始终使用 `weact-cli update` 更新，它会同时更新 CLI 和 AI Skills。

**规则**：不要静默忽略更新提示。即使当前任务与更新无关，也应在完成用户请求后补充告知。

## 安全规则

- **禁止输出密钥**（appSecret、accessToken）到终端明文。
- **写入/删除操作前必须确认用户意图**。
- 用 `--dry-run` 预览危险请求。
- **文件路径只接受相对路径**：`--file`、`--output`、`--output-dir`、`@file` 等路径参数只接受 cwd 下的相对路径，传绝对路径会报 `unsafe file path`。数据输入（`@file`、大 JSON）优先用 stdin 传入，避免路径和转义问题。

## 高风险操作的审批协议（exit 10）

weact-cli 对高风险写操作（`risk: "high-risk-write"`）有强制确认门禁。当你不带 `--yes` 调用这类命令时，CLI 会退出码 `10`、并在 stderr 返回如下结构化 envelope：

```json
{
  "ok": false,
  "error": {
    "type": "confirmation_required",
    "message": "drive +delete requires confirmation",
    "hint": "add --yes to confirm",
    "risk": {
      "level": "high-risk-write",
      "action": "drive +delete"
    }
  }
}
```

**遇到这种情况，不要当普通错误放弃。** 按以下流程处理：

1. **识别**：看到子进程 exit code = `10` 且 stderr JSON 里 `error.type == "confirmation_required"`
2. **向用户确认**：把 `error.risk.action` 和关键参数展示给用户，明确告知"这是高风险操作"，等待用户显式同意
3. **用户同意** → 在你**原始 argv 的末尾追加 `--yes`** 后重试
4. **用户拒绝** → 终止流程，不要擅自改写参数或跳过门禁

**绝对不允许**：
- 看到 exit 10 就默认加 `--yes` 静默重试（这等于禁用门禁）
- 把 `confirmation_required` 当网络错误/权限错误处理
- 在用户没明确同意的前提下追加 `--yes` 重试
- 用 `sh -c` 等 shell 方式拼接命令重试——用 `exec.Command(argv...)` 参数数组形式，避免 shell 解析把用户参数当作语法

提前预判：想先让用户 review 危险操作的具体请求，调用时加 `--dry-run`——它不触发门禁，会打印完整请求详情（URL / body / params），你可以把这个预览给用户看过再去真正执行。

### 如何识别一条命令是高风险

- shortcut：`weact-cli <service> +<cmd> --help` 顶部会显示 `Risk: high-risk-write`
- service 命令：`weact-cli schema <service>.<resource>.<method> --format json` 的返回值里 `"risk": "high-risk-write"`
