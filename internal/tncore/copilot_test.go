package tncore

import (
	"testing"
)

func TestExtractCopilotAnswer(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "Stops at Total usage",
			input:  "cmd1\ncmd2\nTotal usage: 123 tokens\nignored",
			expect: "cmd1\ncmd2",
		},
		{
			name:   "Stops at API time",
			input:  "hello\nAPI time: 50ms\nbye",
			expect: "hello",
		},
		{
			name:   "No footer present",
			input:  "line1\nline2",
			expect: "line1\nline2",
		},
		{
			name:   "Empty input",
			input:  "",
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCopilotAnswer(tt.input)
			if got != tt.expect {
				t.Fatalf("got %q want %q", got, tt.expect)
			}
		})
	}
}
