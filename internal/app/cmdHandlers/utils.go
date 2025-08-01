package cmdHandlers

import (
	"fmt"
	"strconv"
	"strings"
)

// formatOptions turns the list of options into numbered lines suitable for a
// Telegram message.
func formatOptions(opts []string) string {
	lines := make([]string, len(opts))
	for i, o := range opts {
		lines[i] = fmt.Sprintf("%d. %s", i+1, o)
	}
	return strings.Join(lines, "\n")
}

// addCustomOption adds the "custom" option to the provided slice if the user
// is allowed to specify their own category.
func addCustomOption(opts []string, allow bool) []string {
	if !allow {
		return opts
	}
	out := make([]string, len(opts)+1)
	copy(out, opts)
	out[len(opts)] = "😇Своя категория"
	return out
}

// parseSelection parses comma or space separated option indexes from the user
// input and returns the corresponding option values up to the provided limit.
func parseSelection(text string, opts []string, limit int) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' })
	out := []string{}
	seen := map[int]bool{}
	for _, f := range fields {
		idx, err := strconv.Atoi(f)
		if err != nil || idx < 1 || idx > len(opts) || seen[idx] {
			continue
		}
		seen[idx] = true
		out = append(out, opts[idx-1])
		if len(out) == limit {
			break
		}
	}
	return out
}

// numberKeyboard builds a keyboard with numeric buttons from 1 to n.
func numberKeyboard(n int) [][]string {
	rows := [][]string{}
	row := []string{}
	for i := 1; i <= n; i++ {
		row = append(row, strconv.Itoa(i))
		if len(row) == 5 {
			rows = append(rows, row)
			row = []string{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return rows
}

// numberKeyboardWithDone builds a numeric keyboard and adds the "Done" button
// as the last row.
func numberKeyboardWithDone(n int) [][]string {
	rows := numberKeyboard(n)
	rows = append(rows, []string{"Готово"})
	return rows
}

// addBack appends a "Back" button to the given keyboard.
func addBack(kb [][]string) [][]string {
	return append(kb, []string{"Назад"})
}

// addBackCancel appends "Back" and "Cancel" buttons to the keyboard.
func addBackCancel(kb [][]string) [][]string {
	return append(kb, []string{"Назад", "Отмена"})
}

// addCancel appends a "Cancel" button to the keyboard.
func addCancel(kb [][]string) [][]string {
	return append(kb, []string{"Отмена"})
}

// addCancel appends a "Cancel" button to the keyboard.
func addCancelDone(kb [][]string) [][]string {
	return append(kb, []string{"Отмена", "Готово"})
}

// addCancel appends a "Cancel" button to the keyboard.
func addBackDone(kb [][]string) [][]string {
	return append(kb, []string{"Назад", "Готово"})
}

func addInfosInfoSelected(infos []string, cs *ConversationState) {
	for _, inf := range infos {
		found := false
		for _, ex := range cs.SelectedInfos {
			if ex == inf {
				found = true
				break
			}
		}
		if !found && len(cs.SelectedInfos) < cs.InfoLimit {
			cs.SelectedInfos = append(cs.SelectedInfos, inf)
		}
	}
}
