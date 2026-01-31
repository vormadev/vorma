// buyer beware
package xyz

import (
	"strings"
)

func MakeEmojiDataURL(emojiStr string) string {
	sb := strings.Builder{}
	sb.WriteString("data:image/svg+xml,")
	sb.WriteString("<svg xmlns='http://www.w3.org/2000/svg' width='48' height='48' viewBox='0 0 16 16'>")
	sb.WriteString("<text x='0' y='14'>")
	sb.WriteString(emojiStr)
	sb.WriteString("</text>")
	sb.WriteString("</svg>")
	return sb.String()
}
