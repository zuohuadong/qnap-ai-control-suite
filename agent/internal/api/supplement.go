package api

import "net/http"

// 使用精确白名单，避免新增接口或任务入口绕过只读边界。
func supplementReadAllowed(method, path string) bool {
	if method == http.MethodGet {
		switch path {
		case "/v1/health", "/v1/shares", "/v1/shares/smb-status",
			"/v1/files/list", "/v1/files/stat":
			return true
		}
	}
	if method == http.MethodPost {
		switch path {
		case "/v1/files/read", "/v1/files/checksum":
			return true
		}
	}
	return false
}
