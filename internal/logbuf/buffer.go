package logbuf

import "strings"

type LineRingBuffer struct {
	maxLines int
	lines    []string
}

func NewLineRingBuffer(max int) *LineRingBuffer {
	return &LineRingBuffer{
		maxLines: max,
		lines:    make([]string, 0, max),
	}
}

func (b *LineRingBuffer) Write(p []byte) (int, error) {
	line := string(p)

	if len(b.lines) >= b.maxLines {
		// drop oldest
		copy(b.lines, b.lines[1:])
		b.lines = b.lines[:b.maxLines-1]
	}

	b.lines = append(b.lines, line)
	return len(p), nil
}

func (b *LineRingBuffer) String() string {
	return strings.Join(b.lines, "")
}
