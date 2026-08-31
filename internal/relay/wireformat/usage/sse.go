package usage

import "bytes"

// sseData 从一行 SSE 文本中取出 data: 之后的 JSON 负载。
// 第二个返回值为 false 表示该行不是可解析的数据行（注释、event 行、[DONE]）。
func sseData(line []byte) ([]byte, bool) {
	line = bytes.TrimSpace(line)
	if !bytes.HasPrefix(line, []byte("data:")) {
		return nil, false
	}
	payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return nil, false
	}
	return payload, true
}
