package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

const defaultStatusDuration = time.Second * 5

func newErrorMsg(text string) tea.Msg {
	return statusMsg{text: text, isErr: true, d: defaultStatusDuration}
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
	if err == nil {
		return nil
	}

	return func() tea.Msg {
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
