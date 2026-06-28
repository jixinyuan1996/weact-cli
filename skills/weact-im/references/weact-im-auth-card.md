# WeAct 授权卡片模板

当检测到 `missing_scopes` 错误需要引导用户授权时，使用此卡片。

## 使用方式

替换 `{missing_scopes}` 和 `{verification_url}` 后，通过 `weact-cli im +messages-send --msg-type interactive --content '<JSON>'` 发送。

- `{missing_scopes}`：缺失的权限列表，每项一行，格式为 `• approval:task`
- `{verification_url}`：`weact-cli auth login --no-wait --json` 返回的 `verification_url` 原值，不做任何修改

## 卡片 JSON

```json
{
  "config": {
    "wide_screen_mode": true
  },
  "header": {
    "template": "blue",
    "title": {
      "tag": "plain_text",
      "content": "🔐 需要授权"
    }
  },
  "elements": [
    {
      "tag": "div",
      "text": {
        "tag": "lark_md",
        "content": "**你的请求需要以下权限：**\n{missing_scopes}\n\n点击下方按钮在 WeAct 内完成授权，**授权后将自动继续处理，无需任何操作**。"
      }
    },
    {
      "tag": "action",
      "actions": [
        {
          "tag": "button",
          "text": {
            "tag": "plain_text",
            "content": "点击授权"
          },
          "type": "primary",
          "url": "{verification_url}"
        }
      ]
    },
    {
      "tag": "note",
      "elements": [
        {
          "tag": "plain_text",
          "content": "授权在 WeAct 内置浏览器中完成，无需打开系统浏览器。"
        }
      ]
    }
  ]
}
```

## 注意事项

- `url` 类型按钮在 WeAct 桌面端和手机端均在内置浏览器中打开，用户已处于登录态，点击"同意"即可完成授权
- 发送时 `--as bot`，确保以应用身份发送消息
- 发送成功后立即在 background 启动 `weact-cli auth login --device-code <device_code>` 轮询，无需等待用户回复
