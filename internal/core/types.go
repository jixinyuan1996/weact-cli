// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package core

import "os"

// LarkBrand represents the Lark platform brand.
// "feishu" targets China-mainland, "lark" targets international,
// "weact" targets WeAct private deployment.
type LarkBrand string

const (
	BrandFeishu LarkBrand = "feishu"
	BrandLark   LarkBrand = "lark"
	BrandWeAct  LarkBrand = "weact"
)

// ParseBrand normalizes a brand string to a LarkBrand constant.
// Unrecognized values default to BrandFeishu.
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

// OAuthTokenV3Path is the unified OAuth 2.0 Token Endpoint path on the accounts
// domain. It serves every grant type (client_credentials for TAT,
// authorization_code / device_code / refresh_token for UAT) and replaces the
// legacy per-token endpoints (e.g. /open-apis/auth/v3/tenant_access_token/internal).
const OAuthTokenV3Path = "/oauth/v3/token"

// Endpoints holds resolved endpoint URLs for different Lark services.
type Endpoints struct {
	Open     string // e.g. "https://open.feishu.cn"
	Accounts string // e.g. "https://accounts.feishu.cn"
	MCP      string // e.g. "https://mcp.feishu.cn"
	AppLink  string // e.g. "https://applink.feishu.cn"
}

// ResolveEndpoints resolves endpoint URLs based on brand.
// For BrandWeAct, endpoints are read from environment variables with sensible
// defaults, allowing private deployment users to customize each endpoint.
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
			Open:     GetenvOrDefault("WEACT_OPEN_ENDPOINT", "https://open.weact.example.com"),
			Accounts: GetenvOrDefault("WEACT_ACCOUNTS_ENDPOINT", "https://accounts.weact.example.com"),
			MCP:      GetenvOrDefault("WEACT_MCP_ENDPOINT", "https://mcp.weact.example.com"),
			AppLink:  GetenvOrDefault("WEACT_APPLINK_ENDPOINT", "https://applink.weact.example.com"),
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

// ResolveOpenBaseURL returns the Open API base URL for the given brand.
func ResolveOpenBaseURL(brand LarkBrand) string {
	return ResolveEndpoints(brand).Open
}

// GetenvOrDefault returns the value of the environment variable named by key,
// or fallback if the variable is empty or not set.
func GetenvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
