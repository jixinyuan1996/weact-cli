// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/larksuite/cli/internal/core"
)

type loopbackCallback struct {
	code  string
	state string
	err   string
}

// RunLoopbackFlow runs the OAuth 2.0 Authorization Code flow with a local
// HTTP callback server. redirectURI must match exactly what is registered in
// the WeAct developer console (e.g. http://127.0.0.1:18789/callback).
func RunLoopbackFlow(ctx context.Context, httpClient *http.Client, appID, appSecret string, brand core.LarkBrand, scope, redirectURI string, errOut io.Writer) *DeviceFlowResult {
	if errOut == nil {
		errOut = io.Discard
	}

	parsed, parseErr := url.Parse(redirectURI)
	if parseErr != nil || parsed.Hostname() == "" {
		return &DeviceFlowResult{OK: false, Message: fmt.Sprintf("回调地址格式无效 %q", redirectURI)}
	}
	listenAddr := parsed.Hostname() + ":" + parsed.Port()
	callbackPath := parsed.Path
	if callbackPath == "" {
		callbackPath = "/"
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return &DeviceFlowResult{OK: false, Message: fmt.Sprintf("无法在 %s 启动本地服务: %v\n请检查端口是否被占用，或在开发者后台修改回调地址后重试", listenAddr, err)}
	}
	defer listener.Close()

	state := loopbackRandomHex(16)
	ep := core.ResolveEndpoints(brand)
	authURL := buildLoopbackAuthURL(ep.Open, appID, scope, state, redirectURI)

	callbackCh := make(chan loopbackCallback, 1)
	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		cb := loopbackCallback{
			code:  q.Get("code"),
			state: q.Get("state"),
			err:   q.Get("error"),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if cb.err != "" {
			fmt.Fprintf(w, "<html><body><h2>授权失败</h2><p>%s</p><p>请关闭此窗口。</p></body></html>", cb.err)
		} else {
			fmt.Fprintf(w, "<html><body><h2>授权成功！</h2><p>请关闭此窗口，返回终端继续操作。</p></body></html>")
		}
		select {
		case callbackCh <- cb:
		default:
		}
	})
	go srv.Serve(listener) //nolint:errcheck
	defer srv.Close()

	fmt.Fprintf(errOut, "\n请在浏览器中完成 WeAct 授权：\n  %s\n\n", authURL)
	if openErr := loopbackOpenBrowser(authURL); openErr != nil {
		fmt.Fprintf(errOut, "[weact-cli] 无法自动打开浏览器，请手动复制上方链接到浏览器中完成授权。\n")
	}
	fmt.Fprintf(errOut, "等待授权完成（5 分钟超时）...\n")

	select {
	case cb := <-callbackCh:
		if cb.err != "" {
			return &DeviceFlowResult{OK: false, Error: cb.err, Message: fmt.Sprintf("授权被拒绝: %s", cb.err)}
		}
		if cb.state != state {
			return &DeviceFlowResult{OK: false, Message: "state 不匹配，请重试"}
		}
		return loopbackExchangeCode(ctx, httpClient, brand, appID, appSecret, cb.code, redirectURI)
	case <-time.After(5 * time.Minute):
		return &DeviceFlowResult{OK: false, Message: "等待授权超时（5 分钟）"}
	case <-ctx.Done():
		return &DeviceFlowResult{OK: false, Message: "授权已取消"}
	}
}

func buildLoopbackAuthURL(openEndpoint, appID, scope, state, redirectURI string) string {
	params := url.Values{}
	params.Set("app_id", appID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)
	if scope != "" {
		params.Set("scope", scope)
	}
	return openEndpoint + "/open-apis/authen/v1/index?" + params.Encode()
}

func loopbackExchangeCode(ctx context.Context, httpClient *http.Client, brand core.LarkBrand, appID, appSecret, code, redirectURI string) *DeviceFlowResult {
	endpoints := ResolveOAuthEndpoints(brand)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", appID)
	form.Set("client_secret", appSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoints.Token, strings.NewReader(form.Encode()))
	if err != nil {
		return &DeviceFlowResult{OK: false, Message: fmt.Sprintf("failed to create token request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return &DeviceFlowResult{OK: false, Message: fmt.Sprintf("token request failed: %v", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &DeviceFlowResult{OK: false, Message: fmt.Sprintf("failed to read token response: %v", err)}
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return &DeviceFlowResult{OK: false, Message: fmt.Sprintf("failed to parse token response (HTTP %d): %v", resp.StatusCode, err)}
	}

	if errStr := getStr(data, "error"); errStr != "" {
		desc := getStr(data, "error_description")
		if desc == "" {
			desc = getStr(data, "msg")
		}
		return &DeviceFlowResult{OK: false, Error: errStr, Message: desc}
	}

	accessToken := getStr(data, "access_token")
	if accessToken == "" {
		return &DeviceFlowResult{OK: false, Message: fmt.Sprintf("no access_token in response (HTTP %d): %s", resp.StatusCode, body)}
	}

	refreshExpiresIn := getInt(data, "refresh_expires_in", 0)
	if refreshExpiresIn == 0 {
		refreshExpiresIn = getInt(data, "refresh_token_expires_in", 604800)
	}

	return &DeviceFlowResult{
		OK: true,
		Token: &DeviceFlowTokenData{
			AccessToken:      accessToken,
			RefreshToken:     getStr(data, "refresh_token"),
			ExpiresIn:        getInt(data, "expires_in", 7200),
			RefreshExpiresIn: refreshExpiresIn,
			Scope:            getStr(data, "scope"),
		},
	}
}

func loopbackRandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func loopbackOpenBrowser(rawURL string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "linux":
		return exec.Command("xdg-open", rawURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
