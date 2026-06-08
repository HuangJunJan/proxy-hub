package convert

import (
	"encoding/json"
	"testing"
)

// 一致性套件：覆盖 OpenAI chat ⇄ Claude messages 的请求/非流式响应转换。
// 套件全绿是开启 relay.enable_cross_dialect 的前置条件（父设计 §10、§14）。

func TestOpenAIReqToClaude(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4o","temperature":0.5,
		"messages":[
			{"role":"system","content":"be brief"},
			{"role":"user","content":"hi"},
			{"role":"assistant","content":"hello","tool_calls":[{"id":"t1","type":"function","function":{"name":"f","arguments":"{\"a\":1}"}}]},
			{"role":"tool","tool_call_id":"t1","content":"result-text"}
		],
		"stop":["STOP"],
		"tools":[{"type":"function","function":{"name":"f","description":"d","parameters":{"type":"object"}}}]
	}`)
	out, err := OpenAIReqToClaude(body)
	if err != nil {
		t.Fatal(err)
	}
	var got claudeReq
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "gpt-4o" {
		t.Errorf("model 应保留: %q", got.Model)
	}
	if got.MaxTokens != defaultMaxTokens {
		t.Errorf("缺省 max_tokens 应为 %d，实际 %d", defaultMaxTokens, got.MaxTokens)
	}
	if claudeSystemText(got.System) != "be brief" {
		t.Errorf("system 应提取为 'be brief'，实际 %q", claudeSystemText(got.System))
	}
	if len(got.StopSequences) != 1 || got.StopSequences[0] != "STOP" {
		t.Errorf("stop 应转为 stop_sequences=[STOP]，实际 %v", got.StopSequences)
	}
	if len(got.Tools) != 1 || got.Tools[0].Name != "f" {
		t.Errorf("tools 应映射，实际 %+v", got.Tools)
	}
	// 消息：user(hi)、assistant(text+tool_use)、user(tool_result)。
	if len(got.Messages) != 3 {
		t.Fatalf("应得 3 条消息，实际 %d：%+v", len(got.Messages), got.Messages)
	}
	asst := got.Messages[1]
	if asst.Role != "assistant" || len(asst.Content) != 2 || asst.Content[1].Type != "tool_use" || asst.Content[1].Name != "f" {
		t.Errorf("assistant 应含 text+tool_use，实际 %+v", asst)
	}
	tr := got.Messages[2]
	if tr.Role != "user" || tr.Content[0].Type != "tool_result" || tr.Content[0].ToolUseID != "t1" {
		t.Errorf("tool 结果应转为 user.tool_result(tool_use_id=t1)，实际 %+v", tr)
	}
}

func TestClaudeReqToOpenAI(t *testing.T) {
	body := []byte(`{
		"model":"claude-3-5","max_tokens":256,"system":"be brief",
		"stop_sequences":["X"],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"hi"}]},
			{"role":"assistant","content":[{"type":"text","text":"sure"},{"type":"tool_use","id":"u1","name":"f","input":{"a":1}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"u1","content":"42"}]}
		],
		"tools":[{"name":"f","description":"d","input_schema":{"type":"object"}}]
	}`)
	out, err := ClaudeReqToOpenAI(body)
	if err != nil {
		t.Fatal(err)
	}
	var got oaiChatReq
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.MaxTokens == nil || *got.MaxTokens != 256 {
		t.Errorf("max_tokens 应为 256，实际 %v", got.MaxTokens)
	}
	if len(got.Tools) != 1 || got.Tools[0].Function.Name != "f" {
		t.Errorf("tools 应映射为 function，实际 %+v", got.Tools)
	}
	// system + user + assistant(tool_calls) + tool。
	roles := []string{}
	for _, m := range got.Messages {
		roles = append(roles, m.Role)
	}
	want := []string{"system", "user", "assistant", "tool"}
	if len(roles) != len(want) {
		t.Fatalf("消息角色序列应为 %v，实际 %v", want, roles)
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Fatalf("消息角色序列应为 %v，实际 %v", want, roles)
		}
	}
	// assistant 的 tool_calls。
	var asst oaiMessage
	for _, m := range got.Messages {
		if m.Role == "assistant" {
			asst = m
		}
	}
	if len(asst.ToolCalls) != 1 || asst.ToolCalls[0].Function.Name != "f" {
		t.Errorf("assistant 应含 tool_calls(f)，实际 %+v", asst.ToolCalls)
	}
}

func TestClaudeRespToOpenAI(t *testing.T) {
	body := []byte(`{
		"id":"msg_1","type":"message","role":"assistant","model":"claude-3-5",
		"content":[{"type":"text","text":"hello"},{"type":"tool_use","id":"u1","name":"f","input":{"a":1}}],
		"stop_reason":"tool_use",
		"usage":{"input_tokens":11,"output_tokens":7}
	}`)
	out, err := ClaudeRespToOpenAI(body)
	if err != nil {
		t.Fatal(err)
	}
	var got oaiChatResp
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Object != "chat.completion" || got.ID != "msg_1" {
		t.Errorf("响应外形不符: %+v", got)
	}
	if len(got.Choices) != 1 {
		t.Fatalf("应得 1 个 choice")
	}
	ch := got.Choices[0]
	if oaiContentText(ch.Message.Content) != "hello" {
		t.Errorf("文本应为 hello，实际 %q", oaiContentText(ch.Message.Content))
	}
	if len(ch.Message.ToolCalls) != 1 || ch.Message.ToolCalls[0].Function.Name != "f" {
		t.Errorf("应含 tool_calls(f)，实际 %+v", ch.Message.ToolCalls)
	}
	if ch.FinishReason != "tool_calls" {
		t.Errorf("stop_reason tool_use 应映射为 finish_reason tool_calls，实际 %q", ch.FinishReason)
	}
	if got.Usage == nil || got.Usage.PromptTokens != 11 || got.Usage.CompletionTokens != 7 || got.Usage.TotalTokens != 18 {
		t.Errorf("usage 映射不符: %+v", got.Usage)
	}
}

func TestOpenAIRespToClaude(t *testing.T) {
	body := []byte(`{
		"id":"cmpl_1","object":"chat.completion","model":"gpt-4o",
		"choices":[{"index":0,"message":{"role":"assistant","content":"hi there"},"finish_reason":"length"}],
		"usage":{"prompt_tokens":5,"completion_tokens":9,"total_tokens":14}
	}`)
	out, err := OpenAIRespToClaude(body)
	if err != nil {
		t.Fatal(err)
	}
	var got claudeResp
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "message" || got.Role != "assistant" || got.ID != "cmpl_1" {
		t.Errorf("响应外形不符: %+v", got)
	}
	if len(got.Content) != 1 || got.Content[0].Type != "text" || got.Content[0].Text != "hi there" {
		t.Errorf("内容块应为 text='hi there'，实际 %+v", got.Content)
	}
	if got.StopReason != "max_tokens" {
		t.Errorf("finish_reason length 应映射为 stop_reason max_tokens，实际 %q", got.StopReason)
	}
	if got.Usage == nil || got.Usage.InputTokens != 5 || got.Usage.OutputTokens != 9 {
		t.Errorf("usage 映射不符: %+v", got.Usage)
	}
}

func TestStopReasonRoundTrip(t *testing.T) {
	pairs := map[string]string{"end_turn": "stop", "max_tokens": "length", "tool_use": "tool_calls"}
	for claudeR, oaiR := range pairs {
		if got := claudeStopToOpenAI(claudeR); got != oaiR {
			t.Errorf("claude %q → openai 期望 %q，实际 %q", claudeR, oaiR, got)
		}
		if got := openAIFinishToClaude(oaiR); got != claudeR {
			t.Errorf("openai %q → claude 期望 %q，实际 %q", oaiR, claudeR, got)
		}
	}
}

func TestContentStringOrArray(t *testing.T) {
	if got := oaiContentText(json.RawMessage(`"plain"`)); got != "plain" {
		t.Errorf("字符串内容应直取，实际 %q", got)
	}
	if got := oaiContentText(json.RawMessage(`[{"type":"text","text":"a"},{"type":"text","text":"b"}]`)); got != "ab" {
		t.Errorf("数组内容应拼接 text，实际 %q", got)
	}
	if got := parseStringOrArray(json.RawMessage(`"one"`)); len(got) != 1 || got[0] != "one" {
		t.Errorf("string stop 应转单元素切片，实际 %v", got)
	}
}
