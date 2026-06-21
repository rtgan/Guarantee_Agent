package tools

import "encoding/json"

// Result 是所有浏览器工具的统一返回值。
// 预期内的工具失败(元素找不到、断言失败)以 OK=false 且带稳定 ErrorCode 的 Result 返回,
// 而非 Go error,这样 agent 循环可把它当作可恢复的观察值处理。
// 只有基础设施级失败(panic、写盘 IO)才以 Go error 形式上抛。
type Result struct {
	OK          bool           `json:"ok"`
	Message     string         `json:"message"`
	Observation string         `json:"observation,omitempty"`
	ErrorCode   string         `json:"errorCode,omitempty"`
	Retriable   bool           `json:"retriable,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

// OK 构造一个成功结果,可附带供模型/IR 使用的结构化数据。
func OK(message string, data map[string]any) Result {
	return Result{OK: true, Message: message, Data: data}
}

// Fail 构造一个失败结果,带稳定错误码和是否可重试的提示。
func Fail(code, message string, retriable bool) Result {
	return Result{OK: false, ErrorCode: code, Message: message, Retriable: retriable}
}

// String 把结果序列化为 JSON;Eino 把它作为 tool 消息内容回传给模型。
func (r Result) String() string {
	b, _ := json.Marshal(r)
	return string(b)
}
