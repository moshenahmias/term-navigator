package tncore

import (
	"reflect"
	"testing"
)

func TestExtractStringArray(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:   "Simple array",
			input:  `["a","b","c"]`,
			expect: []string{"a", "b", "c"},
		},
		{
			name:   "Whitespace",
			input:  ` [ "x" , "y" ] `,
			expect: []string{"x", "y"},
		},
		{
			name:   "Empty array",
			input:  `[]`,
			expect: []string{},
		},
		{
			name:   "Invalid JSON",
			input:  `not json`,
			expect: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractStringArray(tt.input)
			if !reflect.DeepEqual(got, tt.expect) {
				t.Fatalf("got %#v want %#v", got, tt.expect)
			}
		})
	}
}
