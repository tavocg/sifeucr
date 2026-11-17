package home

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func avatarInitial(email string) string {
	first, _ := utf8.DecodeRuneInString(email)
	return strings.ToUpper(string(first))
}

func avatarColor(initial string) string {
	letter, _ := utf8.DecodeRuneInString(initial)
	return fmt.Sprintf("hsl(%d 55%% 32%%)", uint32(letter)*137%360)
}

func current(active bool) string {
	if active {
		return "page"
	}
	return "false"
}
