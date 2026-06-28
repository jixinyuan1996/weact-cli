// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"io"
	"strings"
)

// weactScopeDesc maps WeAct/Feishu scope names to human-readable Chinese descriptions.
var weactScopeDesc = map[string]string{
	"auth:user.id:read":               "读取自己的用户信息（姓名、头像、open_id）",
	"im:chat:readonly":                "读取群列表与群信息",
	"im:message:readonly":             "读取消息记录（受租户策略限制，可能无法生效）",
	"im:message":                      "发送消息（受租户策略限制，可能无法生效）",
	"im:message.group_at_msg:readonly": "读取群@消息",
	"im:message.p2p_msg:readonly":     "读取私信消息",
	"approval:approval":               "查看与操作审批（需以 bot 身份调用）",
	"contact:user.basic_profile:readonly": "读取用户基本资料",
	"contact:user:search":             "搜索用户",
	"profile:user_profile:read":       "读取用户档案",
	"drive:drive:readonly":            "读取云文档",
	"calendar:calendar:readonly":      "读取日历",
	"task:task:read":                  "读取任务",
	"task:task:write":                 "创建与更新任务",
}

// weactTenantPolicyWarning lists scopes that WeAct tenant policy commonly restricts.
var weactTenantPolicyWarning = map[string]bool{
	"im:message:readonly": true,
	"im:message":          true,
}

// writeWeActCapabilitySummary prints a human-friendly summary of what the user
// can actually do with their granted scopes on WeAct private deployment.
func writeWeActCapabilitySummary(errOut io.Writer, grantedScopes string) {
	scopes := strings.Fields(grantedScopes)
	if len(scopes) == 0 {
		return
	}

	fmt.Fprintln(errOut, "\n── WeAct 授权能力说明 ──────────────────────────")
	fmt.Fprintln(errOut, "当前已授予的权限及可用功能：")

	for _, scope := range scopes {
		desc, ok := weactScopeDesc[scope]
		if !ok {
			desc = scope
		}
		if weactTenantPolicyWarning[scope] {
			fmt.Fprintf(errOut, "  ⚠  %s\n", desc)
		} else {
			fmt.Fprintf(errOut, "  ✓  %s\n", desc)
		}
	}

	hasWarning := false
	for _, scope := range scopes {
		if weactTenantPolicyWarning[scope] {
			hasWarning = true
			break
		}
	}
	if hasWarning {
		fmt.Fprintln(errOut, "\n  ⚠  标注的权限在管网 WeAct 中受租户策略限制，即使授权成功也可能无法通过 API 使用。")
		fmt.Fprintln(errOut, "     如需解锁，请联系 WeAct 管理员开放对应策略。")
	}

	fmt.Fprintln(errOut, "\n提示：可运行 `weact-cli auth status` 查看完整授权状态。")
	fmt.Fprintln(errOut, "────────────────────────────────────────────────")
}
