package adaptor

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"strings"
)

// CopyStatusAndHeaders 把上游响应的状态码与安全头复制到客户端响应（跳过逐跳头与 Content-Length）。
func CopyStatusAndHeaders(w http.ResponseWriter, resp *http.Response) {
	for k, vs := range resp.Header {
		if skipHeader(k) {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
}

// BufferCopy 读取整个上游响应体写给客户端，并返回字节用于解析 usage（非流式）。
func BufferCopy(w http.ResponseWriter, resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_, werr := w.Write(body)
	return body, werr
}

// StreamCopy 逐行把 SSE 响应透传给客户端并立即 flush；对每个 `data:` 负载调用 onData（用于嗅探 usage）。
// onData 可为 nil。透传字节原样，不改写响应内容。
func StreamCopy(w http.ResponseWriter, resp *http.Response, onData func(payload []byte)) error {
	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := w.Write(line); werr != nil {
				return werr
			}
			if flusher != nil {
				flusher.Flush()
			}
			if onData != nil {
				trimmed := bytes.TrimSpace(line)
				if bytes.HasPrefix(trimmed, []byte("data:")) {
					payload := bytes.TrimSpace(trimmed[len("data:"):])
					if len(payload) > 0 && !bytes.Equal(payload, []byte("[DONE]")) {
						onData(payload)
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// IsEventStream 判断上游响应是否为 SSE 流。
func IsEventStream(resp *http.Response) bool {
	return strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
}

// skipHeader 报告该响应头是否应在透传时跳过（逐跳头 + Content-Length 由写入方重新决定）。
func skipHeader(k string) bool {
	switch http.CanonicalHeaderKey(k) {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
		"Te", "Trailer", "Transfer-Encoding", "Upgrade", "Content-Length":
		return true
	}
	return false
}
