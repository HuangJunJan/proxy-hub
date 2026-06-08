// Package convert 实现 OpenAI chat ⇄ Claude messages 的跨方言转换（请求 + 非流式响应 + 流式见 sse.go）。
//
// 仅承载方言转换的承重逻辑，用类型化 struct 往返（未知/多模态字段尽量透传或忽略并在注释标注限制）。
// 由 relay 在「入站方言与上游平台不一致」且特性开关 relay.enable_cross_dialect 为真时调用；
// 默认关闭，一致性套件（convert_test.go）验证绿后再开（见父设计 §10、§14）。
package convert

import (
	"encoding/json"
	"fmt"
	"strings"
)

// defaultMaxTokens 是 OpenAI→Claude 时 max_tokens 缺省值（Claude 要求该字段必填）。
const defaultMaxTokens = 4096

// ---- OpenAI chat 类型（子集） ----

type oaiChatReq struct {
	Model       string          `json:"model"`
	Messages    []oaiMessage    `json:"messages"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stop        json.RawMessage `json:"stop,omitempty"` // string 或 []string
	Stream      bool            `json:"stream,omitempty"`
	Tools       []oaiTool       `json:"tools,omitempty"`
}

type oaiMessage struct {
	Role       string          `json:"role"` // system|user|assistant|tool
	Content    json.RawMessage `json:"content,omitempty"`
	ToolCalls  []oaiToolCall   `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type oaiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"` // "function"
	Function oaiFunctionCall `json:"function"`
}

type oaiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串
}

type oaiTool struct {
	Type     string         `json:"type"` // "function"
	Function oaiFunctionDef `json:"function"`
}

type oaiFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type oaiChatResp struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Model   string      `json:"model"`
	Choices []oaiChoice `json:"choices"`
	Usage   *oaiUsage   `json:"usage,omitempty"`
}

type oaiChoice struct {
	Index        int        `json:"index"`
	Message      oaiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type oaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ---- Claude messages 类型（子集） ----

type claudeReq struct {
	Model         string          `json:"model"`
	System        json.RawMessage `json:"system,omitempty"` // string 或 []block
	Messages      []claudeMessage `json:"messages"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Tools         []claudeTool    `json:"tools,omitempty"`
}

type claudeMessage struct {
	Role    string        `json:"role"` // user|assistant
	Content []claudeBlock `json:"content"`
}

type claudeBlock struct {
	Type string `json:"type"` // text|tool_use|tool_result
	Text string `json:"text,omitempty"`
	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// tool_result
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"` // string 或 []block
	IsError   bool            `json:"is_error,omitempty"`
}

type claudeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type claudeResp struct {
	ID         string        `json:"id"`
	Type       string        `json:"type"` // "message"
	Role       string        `json:"role"` // "assistant"
	Model      string        `json:"model"`
	Content    []claudeBlock `json:"content"`
	StopReason string        `json:"stop_reason"`
	Usage      *claudeUsage  `json:"usage,omitempty"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ---- 请求转换 ----

// OpenAIReqToClaude 把 OpenAI chat 请求体转为 Claude messages 请求体。
func OpenAIReqToClaude(body []byte) ([]byte, error) {
	var in oaiChatReq
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 请求失败: %w", err)
	}
	out := claudeReq{
		Model:       in.Model,
		Temperature: in.Temperature,
		TopP:        in.TopP,
		Stream:      in.Stream,
	}
	if in.MaxTokens != nil {
		out.MaxTokens = *in.MaxTokens
	} else {
		out.MaxTokens = defaultMaxTokens
	}
	out.StopSequences = parseStringOrArray(in.Stop)

	for _, t := range in.Tools {
		out.Tools = append(out.Tools, claudeTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	var systemParts []string
	for _, m := range in.Messages {
		switch m.Role {
		case "system":
			systemParts = append(systemParts, oaiContentText(m.Content))
		case "user":
			out.Messages = append(out.Messages, claudeMessage{
				Role:    "user",
				Content: []claudeBlock{{Type: "text", Text: oaiContentText(m.Content)}},
			})
		case "assistant":
			blocks := []claudeBlock{}
			if txt := oaiContentText(m.Content); txt != "" {
				blocks = append(blocks, claudeBlock{Type: "text", Text: txt})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, claudeBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: rawOrEmptyObject(tc.Function.Arguments),
				})
			}
			out.Messages = append(out.Messages, claudeMessage{Role: "assistant", Content: blocks})
		case "tool":
			// OpenAI 的 tool 结果消息 → Claude user 消息内的 tool_result 块。
			out.Messages = append(out.Messages, claudeMessage{
				Role: "user",
				Content: []claudeBlock{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   jsonString(oaiContentText(m.Content)),
				}},
			})
		}
	}
	if len(systemParts) > 0 {
		out.System = jsonString(strings.Join(systemParts, "\n\n"))
	}

	return json.Marshal(out)
}

// ClaudeReqToOpenAI 把 Claude messages 请求体转为 OpenAI chat 请求体。
func ClaudeReqToOpenAI(body []byte) ([]byte, error) {
	var in claudeReq
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, fmt.Errorf("解析 Claude 请求失败: %w", err)
	}
	mt := in.MaxTokens
	out := oaiChatReq{
		Model:       in.Model,
		Temperature: in.Temperature,
		TopP:        in.TopP,
		Stream:      in.Stream,
	}
	if mt > 0 {
		out.MaxTokens = &mt
	}
	if len(in.StopSequences) > 0 {
		out.Stop, _ = json.Marshal(in.StopSequences)
	}
	for _, t := range in.Tools {
		out.Tools = append(out.Tools, oaiTool{
			Type:     "function",
			Function: oaiFunctionDef{Name: t.Name, Description: t.Description, Parameters: t.InputSchema},
		})
	}

	// system（顶层）→ 一条 system 消息。
	if sys := claudeSystemText(in.System); sys != "" {
		out.Messages = append(out.Messages, oaiMessage{Role: "system", Content: jsonString(sys)})
	}
	for _, m := range in.Messages {
		switch m.Role {
		case "user":
			// user 块可能含 text 与 tool_result：tool_result 拆成独立的 OpenAI tool 消息。
			var texts []string
			for _, b := range m.Content {
				switch b.Type {
				case "text":
					texts = append(texts, b.Text)
				case "tool_result":
					out.Messages = append(out.Messages, oaiMessage{
						Role:       "tool",
						ToolCallID: b.ToolUseID,
						Content:    jsonString(claudeBlockContentText(b.Content)),
					})
				}
			}
			if len(texts) > 0 {
				out.Messages = append(out.Messages, oaiMessage{Role: "user", Content: jsonString(strings.Join(texts, "\n"))})
			}
		case "assistant":
			var texts []string
			var toolCalls []oaiToolCall
			for _, b := range m.Content {
				switch b.Type {
				case "text":
					texts = append(texts, b.Text)
				case "tool_use":
					toolCalls = append(toolCalls, oaiToolCall{
						ID:       b.ID,
						Type:     "function",
						Function: oaiFunctionCall{Name: b.Name, Arguments: rawToString(b.Input)},
					})
				}
			}
			msg := oaiMessage{Role: "assistant"}
			if len(texts) > 0 {
				msg.Content = jsonString(strings.Join(texts, "\n"))
			}
			msg.ToolCalls = toolCalls
			out.Messages = append(out.Messages, msg)
		}
	}
	return json.Marshal(out)
}

// ---- 非流式响应转换 ----

// ClaudeRespToOpenAI 把 Claude messages 响应体转为 OpenAI chat completion 响应体。
func ClaudeRespToOpenAI(body []byte) ([]byte, error) {
	var in claudeResp
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, fmt.Errorf("解析 Claude 响应失败: %w", err)
	}
	msg := oaiMessage{Role: "assistant"}
	var texts []string
	var toolCalls []oaiToolCall
	for _, b := range in.Content {
		switch b.Type {
		case "text":
			texts = append(texts, b.Text)
		case "tool_use":
			toolCalls = append(toolCalls, oaiToolCall{
				ID:       b.ID,
				Type:     "function",
				Function: oaiFunctionCall{Name: b.Name, Arguments: rawToString(b.Input)},
			})
		}
	}
	if len(texts) > 0 {
		msg.Content = jsonString(strings.Join(texts, ""))
	}
	msg.ToolCalls = toolCalls

	out := oaiChatResp{
		ID:      in.ID,
		Object:  "chat.completion",
		Model:   in.Model,
		Choices: []oaiChoice{{Index: 0, Message: msg, FinishReason: claudeStopToOpenAI(in.StopReason)}},
	}
	if in.Usage != nil {
		out.Usage = &oaiUsage{
			PromptTokens:     in.Usage.InputTokens,
			CompletionTokens: in.Usage.OutputTokens,
			TotalTokens:      in.Usage.InputTokens + in.Usage.OutputTokens,
		}
	}
	return json.Marshal(out)
}

// OpenAIRespToClaude 把 OpenAI chat completion 响应体转为 Claude messages 响应体。
func OpenAIRespToClaude(body []byte) ([]byte, error) {
	var in oaiChatResp
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 响应失败: %w", err)
	}
	out := claudeResp{
		ID:      in.ID,
		Type:    "message",
		Role:    "assistant",
		Model:   in.Model,
		Content: []claudeBlock{},
	}
	if len(in.Choices) > 0 {
		ch := in.Choices[0]
		if txt := oaiContentText(ch.Message.Content); txt != "" {
			out.Content = append(out.Content, claudeBlock{Type: "text", Text: txt})
		}
		for _, tc := range ch.Message.ToolCalls {
			out.Content = append(out.Content, claudeBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: rawOrEmptyObject(tc.Function.Arguments),
			})
		}
		out.StopReason = openAIFinishToClaude(ch.FinishReason)
	}
	if in.Usage != nil {
		out.Usage = &claudeUsage{InputTokens: in.Usage.PromptTokens, OutputTokens: in.Usage.CompletionTokens}
	}
	return json.Marshal(out)
}

// ---- 停止原因映射 ----

func claudeStopToOpenAI(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

func openAIFinishToClaude(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	default:
		return "end_turn"
	}
}

// ---- 内容/字段辅助 ----

// oaiContentText 从 OpenAI content（string 或 [{type:text,text}...]）提取纯文本。
func oaiContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Type == "text" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	return ""
}

// claudeSystemText 从 Claude system（string 或 [{type:text,text}...]）提取纯文本。
func claudeSystemText(raw json.RawMessage) string {
	return oaiContentText(raw) // 形状相同，复用。
}

// claudeBlockContentText 从 tool_result 的 content（string 或 [{type:text,text}...]）提取纯文本。
func claudeBlockContentText(raw json.RawMessage) string {
	return oaiContentText(raw)
}

// parseStringOrArray 把 string 或 []string 的原始 JSON 解析为字符串切片。
func parseStringOrArray(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

// jsonString 把字符串编码为 JSON 字符串原文（用于 RawMessage 字段）。
func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// rawToString 把原始 JSON（对象）转为其字符串表示（OpenAI tool 参数是 JSON 字符串）。
func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}

// rawOrEmptyObject 把 OpenAI 的 arguments JSON 字符串解析为对象 RawMessage；空/非法回退 {}。
func rawOrEmptyObject(args string) json.RawMessage {
	s := strings.TrimSpace(args)
	if s == "" {
		return json.RawMessage(`{}`)
	}
	if !json.Valid([]byte(s)) {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(s)
}
