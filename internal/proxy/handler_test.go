package proxy

import "testing"

func TestParseUsageSupportsResponsesTokenNames(t *testing.T) {
	prompt, completion, total := parseUsage([]byte(`{"usage":{"input_tokens":7,"output_tokens":11,"total_tokens":18}}`))
	if prompt == nil || *prompt != 7 {
		t.Fatalf("prompt tokens = %v, want 7", prompt)
	}
	if completion == nil || *completion != 11 {
		t.Fatalf("completion tokens = %v, want 11", completion)
	}
	if total == nil || *total != 18 {
		t.Fatalf("total tokens = %v, want 18", total)
	}
}
