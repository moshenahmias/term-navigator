package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	defaultFastStatusDuration = time.Second / 2
	defaultStatusDuration     = time.Second * 5
	defaultErrorDuration      = time.Second * 10
)

func newLongStatusMsg(lines ...string) tea.Msg {
	if len(lines) == 0 {
		return clearStatusMsg{}
	}

	msg := statusMsg{text: lines[0], isErr: false, d: defaultFastStatusDuration}

	p := &msg

	for _, s := range lines[1:] {
		p.next = &statusMsg{text: s, isErr: false, d: defaultFastStatusDuration}
		p = p.next
	}

	return msg
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
