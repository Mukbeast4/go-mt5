package gomt5

import "strings"

func matchGroup(symbol, group string) bool {
	if group == "" {
		return true
	}
	if strings.HasSuffix(group, "*") {
		prefix := strings.TrimSuffix(group, "*")
		return strings.HasPrefix(symbol, prefix)
	}
	return symbol == group
}

func filterByGroup[T any](items []T, group string, symbol func(T) string) []T {
	filtered := make([]T, 0, len(items))
	for _, item := range items {
		if matchGroup(symbol(item), group) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
