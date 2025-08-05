package cmdHandlers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	DoneButton = "Готово"

	DeleteEverything = "Удалить все"
	DeleteSome       = "Удалить 1 и более"

	UpdateEverything = "Обновить все"
	UpdateSome       = "Обновить 1 и более"

	CustomCatName = "😇Своя категория"
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

// parseSelection parse answer from keyboard
func parseSelectionOne(text string, opts []string) string {
	idx, err := strconv.Atoi(text)
	if err != nil || idx < 1 || idx > len(opts) {
		return ""
	}

	return opts[idx-1]
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

func addSelectedInfo(infos []string, cs *ConversationState) {
	for _, inf := range infos {
		found := false
		for _, ex := range cs.TopicsConvP.SelectedInfos {
			if ex == inf {
				found = true
				break
			}
		}
		if !found && len(cs.TopicsConvP.SelectedInfos) < cs.TopicsConvP.InfoLimit {
			cs.TopicsConvP.SelectedInfos = append(cs.TopicsConvP.SelectedInfos, inf)
		}
	}
}

func buildAlreadyChosenInfos(topics map[string][]string) string {
	keys := make([]string, 0, len(topics))
	for k := range topics {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	b := strings.Builder{}
	for _, cat := range keys {
		infos := topics[cat]
		if len(infos) == 0 {
			b.WriteString(cat + ": [ ]\n\t")
			continue
		}
		b.WriteString(cat + ": [<u>")

		for i, info := range infos {
			if i == len(infos)-1 {
				b.WriteString(info)
				continue
			}
			b.WriteString(info + ", ")
		}
		b.WriteString("</u>]\n\t")
	}
	return b.String()
}

func getKeys(mapa map[string][]string) []string {
	keys := make([]string, 0, len(mapa))
	for k := range mapa {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}

func deleteKeysFromMap(mapa map[string][]string, dkeys []string) {
	for _, dk := range dkeys {
		if _, isExist := mapa[dk]; isExist {
			delete(mapa, dk)
		}
	}
}

func getNextCustomCat(cats []string, number int) string {
	for _, cat := range cats {
		if strings.Contains(cat, CustomCatName) && strings.HasSuffix(cat, "_"+strconv.Itoa(number)) {
			return cat
		}
	}

	return ""
}

// removeWithSubstring удаляет все элементы, которые содержат подстроку "бла"
func removeCustomCats(slice []string) []string {
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if !strings.Contains(v, CustomCatName) {
			result = append(result, v)
		}
	}
	return result
}
