package tncore

import (
	"fmt"
	"strings"
)

func extractCopilotAnswer(raw string) string {
	lines := strings.Split(raw, "\n")

	var answer []string
	for _, line := range lines {
		if strings.HasPrefix(line, "Total usage") ||
			strings.HasPrefix(line, "API time") ||
			strings.HasPrefix(line, "Total session") ||
			strings.HasPrefix(line, "Breakdown by") {
			break
		}
		answer = append(answer, line)
	}

	return strings.TrimSpace(strings.Join(answer, "\n"))
}

func (a *App) generateCopilotPrompt(input string) string {
	src, dst := a.panes()

	var sb strings.Builder

	sb.WriteString("You are operating inside a two-pane file explorer.\n")

	if src == a.left {
		sb.WriteString("The LEFT pane is the active (source) pane. The RIGHT pane is the destination pane.\n")
	} else {
		sb.WriteString("The RIGHT pane is the active (source) pane. The LEFT pane is the destination pane.\n")
	}

	fmt.Fprintf(&sb, "Source pane explorer type: %s\n", src.explorer.Type())
	fmt.Fprintf(&sb, "Destination pane explorer type: %s\n", dst.explorer.Type())

	fmt.Fprintf(&sb, "Source pane device name: %s\n", src.name)
	fmt.Fprintf(&sb, "Destination pane device name: %s\n", dst.name)

	sb.WriteString("Source pane items:\n")
	for _, v := range src.list.Items() {
		item := v.(*FileItem)
		fmt.Fprintf(&sb, "%v\n", item.Info)
	}

	sb.WriteString("Destination pane items:\n")
	for _, v := range dst.list.Items() {
		item := v.(*FileItem)
		fmt.Fprintf(&sb, "%v\n", item.Info)
	}

	sb.WriteString("Available devices:\n")
	for name, dev := range a.devs {
		fmt.Fprintf(&sb, "Device name: %s, type: %s\n", name, dev.Type())
	}

	sb.WriteString("Available commands:\n")
	sb.WriteString("rename <item> <new name>        - rename an item in the active pane\n")
	sb.WriteString("view <item>                     - view an item in the active pane\n")
	sb.WriteString("delete <item>                   - delete an item in the active pane\n")
	sb.WriteString("edit <item>                     - edit an item in the active pane\n")
	sb.WriteString("copy <src item> <dst name>      - copy an item from the active pane to the destination pane\n")
	sb.WriteString("move <src item> <dst name>      - move an item from the active pane to the destination pane\n")
	sb.WriteString("mkdir <folder name>             - create a folder in the active pane\n")
	sb.WriteString("device <device name>            - set the active pane's device\n")
	sb.WriteString("switch                          - switch which pane is active\n")
	sb.WriteString("cd <folder name>                - change directory inside the active pane\n")

	sb.WriteString("In all commands, when you refer to a folder, use / at the end\n")
	sb.WriteString("I will now ask you something. Based on all the information above, respond ONLY with valid commands.\n")
	sb.WriteString("If you need to refer to an item inside a folder, you MUST cd into that folder first. Commands operate ONLY on the current directories of both panes. Full paths are NOT allowed.\n")
	sb.WriteString("Your answer MUST be EXACTLY in this format, with no extra text: [\"cmd...\", \"cmd...\", ...]\n")

	return sb.String() + input
}
