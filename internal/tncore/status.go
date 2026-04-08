package tncore

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	defaultFastStatusDuration = time.Second / 2
	defaultStatusDuration     = time.Second * 5
	defaultErrorDuration      = time.Second * 10
	defaultFastErrorDuration  = time.Second * 3
)

func wrapLine(line string, width int) []string {
	if width <= 0 || len(line) <= width {
		return []string{line}
	}

	var out []string
	for len(line) > width {
		out = append(out, line[:width])
		line = line[width:]
	}
	if len(line) > 0 {
		out = append(out, line)
	}
	return out
}

func splitStatusMsgLines(msg statusMsg, width int) statusMsg {
	// Split into raw lines
	raw := strings.Split(msg.text, "\n")

	var lines []string
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Wrap long lines
		wrapped := wrapLine(line, width)
		lines = append(lines, wrapped...)
	}

	if len(lines) == 0 {
		return statusMsg{}
	}

	// If only one line, return as-is
	if len(lines) == 1 {
		msg.text = lines[0]
		return msg
	}

	// Build linked list
	head := statusMsg{
		text:  lines[0],
		isErr: msg.isErr,
		d:     msg.d,
	}

	p := &head
	for _, line := range lines[1:] {
		p.next = &statusMsg{
			text:  line,
			isErr: msg.isErr,
			d:     msg.d,
		}
		p = p.next
	}

	return head
}

func newLongStatusOrErrorMsg(isErr bool, lines ...string) tea.Msg {
	if len(lines) == 0 {
		return clearStatusMsg{}
	}

	d := defaultFastStatusDuration

	if isErr {
		d = defaultFastErrorDuration
	}

	msg := statusMsg{text: lines[0], isErr: isErr, d: d}

	p := &msg

	for _, s := range lines[1:] {
		p.next = &statusMsg{text: s, isErr: isErr, d: d}
		p = p.next
	}

	return msg
}

func NewLongStatusMsg(lines ...string) tea.Msg {
	return newLongStatusOrErrorMsg(false, lines...)
}

func NewLongErrorMsg(lines ...string) tea.Msg {
	return newLongStatusOrErrorMsg(true, lines...)
}

func NewLongErrorMsgFromErrors(errs ...error) tea.Msg {
	var lines []string

	for _, err := range errs {
		if err != nil {
			lines = append(lines, err.Error())
		}
	}

	return NewLongErrorMsg(lines...)
}

func newErrorMsg(text string) tea.Msg {
	return statusMsg{text: text, isErr: true, d: defaultErrorDuration}
}

func newStatusMsg(text string) tea.Msg {
	return statusMsg{text: text, isErr: false, d: defaultStatusDuration}
}

func failure(text string) tea.Cmd {
	return func() tea.Msg {
		return newErrorMsg(text)
	}
}

func failuref(format string, a ...any) tea.Cmd {
	return failure(fmt.Sprintf(format, a...))
}

func execCheck() tea.ExecCallback {
	return func(err error) tea.Msg {
		if err == nil {
			return nil
		}
		return newErrorMsg(err.Error())
	}
}

func execResolve(text string) tea.ExecCallback {
	return func(err error) tea.Msg {
		if err == nil {
			return newStatusMsg(text)
		}
		return newErrorMsg(err.Error())
	}
}

func check(err error) tea.Cmd {
	return func() tea.Msg {
		if err == nil {
			return nil
		}

		return newErrorMsg(err.Error())
	}
}

func resolve(err error, or string) tea.Cmd {
	return func() tea.Msg {
		if err == nil {
			return newStatusMsg(or)
		}
		return newErrorMsg(err.Error())
	}
}

func status(text string) tea.Cmd {
	return resolve(nil, text)
}

func statusf(format string, a ...any) tea.Cmd {
	return status(fmt.Sprintf(format, a...))
}
