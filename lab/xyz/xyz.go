package xyz

import (
	"net/url"
	"strings"
)

func MakeEmojiDataURL(emojiStr string) string {
	sb := strings.Builder{}
	sb.WriteString(
		"<svg xmlns='http://www.w3.org/2000/svg' width='48' height='48' viewBox='0 0 16 16'>",
	)
	sb.WriteString("<text x='0' y='14'>")
	sb.WriteString(emojiStr)
	sb.WriteString("</text>")
	sb.WriteString("</svg>")

	return "data:image/svg+xml," + url.PathEscape(sb.String())
}
