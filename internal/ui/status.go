package ui

import tea "charm.land/bubbletea/v2"

func newErrorMsg(text string) tea.Msg {
	return statusMsg{text: text, isErr: true}
}

func newStatusMsg(text string) tea.Msg {
	return statusMsg{text: text, isErr: false}
}

func failure(text string) tea.Cmd {
	return func() tea.Msg {
		return newErrorMsg(text)
	}
}

func execCheck() tea.ExecCallback {
	return func(err error) tea.Msg {
		return check(err)
	}
}

func execResolve(text string) tea.ExecCallback {
	return func(err error) tea.Msg {
		return resolve(err, text)
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
