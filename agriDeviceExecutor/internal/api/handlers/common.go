package handlers

import (
	"agriDeviceExecutor/internal/config"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// writeJSON 以给定的 HTTP 状态码将 payload 写为 JSON 响应。
//
// 行为说明：
// - 设置 Content-Type 为 application/json; charset=utf-8
// - 使用 json.Encoder 流式编码，避免构建过大的中间字节切片
// - 在编码前写入状态码，符合 HTTP 发送顺序
// - 编码错误不在此处再次写入错误内容，避免输出混乱（如需日志请在调用方记录）
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// decodeJSON 将请求体 JSON 解码到 v。
//
// 加固策略：
// - DisallowUnknownFields：拒绝未知字段，防止客户端静默传入多余字段
// - 解码后检查是否还有残留数据，避免尾随垃圾数据
// - r.Body 的关闭由 http 服务器负责，调用方无需手动关闭
func decodeJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	// Ensure no trailing data remains
	if dec.More() {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// OnlyGet 包装处理函数，仅允许 GET 方法访问。
// 对于其它方法，返回 405 Method Not Allowed，并正确设置 Allow 头。
func OnlyGet(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

// OnlyPost 包装处理函数，仅允许 POST 方法访问。
// 对于其它方法，返回 405 Method Not Allowed，并正确设置 Allow 头。
func OnlyPost(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

// RequireAuth 中间件：严格获取本地 token + 规范化基础地址（去除尾部斜杠）。
// 失败：401 未登录 / 500 其它错误；成功：调用下游并传递 token 与 baseURL。
func RequireAuth(h func(http.ResponseWriter, *http.Request, string, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 严格 token
		token, err := config.GetUserToken()
		if err != nil {
			if errors.Is(err, config.ErrTokenMissing) {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"code": 401, "message": "未登录"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
		// 基础地址（严格版：若读取失败直接 500）
		baseURL, err := config.GetNormalizedAPIBaseURL()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
		h(w, r, token, baseURL)
	}
}
