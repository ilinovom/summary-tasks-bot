package utils

import "sort"

func GetSortedKeys(mapa map[string][]string) []string {
	keys := make([]string, 0, len(mapa))
	for k := range mapa {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}
