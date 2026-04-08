package tncore

import (
	"reflect"
	"testing"
)

func TestWrapLine(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		width  int
		expect []string
	}{
		{
			name:   "ASCII simple wrap",
			input:  "HelloWorld",
			width:  5,
			expect: []string{"Hello", "World"},
		},
		{
			name:   "Unicode emoji wrap",
			input:  "🙂🙂🙂🙂",
			width:  2,
			expect: []string{"🙂🙂", "🙂🙂"},
		},
		{
			name:   "Exact width no wrap",
			input:  "Hello",
			width:  5,
			expect: []string{"Hello"},
		},
		{
			name:   "Width larger than string",
			input:  "Hi",
			width:  10,
			expect: []string{"Hi"},
		},
		{
			name:   "Zero width returns whole line",
			input:  "Hello",
			width:  0,
			expect: []string{"Hello"},
		},
		{
			name:   "Single rune width",
			input:  "abcd",
			width:  1,
			expect: []string{"a", "b", "c", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapLine(tt.input, tt.width)
			if !reflect.DeepEqual(got, tt.expect) {
				t.Fatalf("wrapLine(%q, %d) = %#v, want %#v",
					tt.input, tt.width, got, tt.expect)
			}
		})
	}
}

func TestSplitStatusMsgLines(t *testing.T) {
	tests := []struct {
		name     string
		msg      statusMsg
		width    int
		expected []string // flattened linked list
	}{
		{
			name:     "Preserve leading spaces",
			msg:      statusMsg{text: "   indented line"},
			width:    20,
			expected: []string{"   indented line"},
		},
		{
			name:     "Preserve empty lines",
			msg:      statusMsg{text: "line1\n\nline3"},
			width:    20,
			expected: []string{"line1", "", "line3"},
		},
		{
			name:     "Unicode wrapping",
			msg:      statusMsg{text: "🙂🙂🙂🙂"},
			width:    2,
			expected: []string{"🙂🙂", "🙂🙂"},
		},
		{
			name:     "Multiple wrapped lines",
			msg:      statusMsg{text: "HelloWorld"},
			width:    5,
			expected: []string{"Hello", "World"},
		},
		{
			name:     "Single short line",
			msg:      statusMsg{text: "short"},
			width:    10,
			expected: []string{"short"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			head := splitStatusMsgLines(tt.msg, tt.width)

			// Flatten linked list WITHOUT skipping empty lines
			var got []string
			for p := &head; p != nil; p = p.next {
				got = append(got, p.text)
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Fatalf("splitStatusMsgLines() = %#v, want %#v", got, tt.expected)
			}
		})
	}
}
