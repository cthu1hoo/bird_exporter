package parser

import (
	"regexp"
	"strings"
)

// replyCodeRegex matches the BIRD CLI reply code that prefixes output lines,
// for example "1002-ospf1:" or "2002-Router ID". bird_socket returns the raw
// reply, so every parser has to look past it.
var replyCodeRegex = regexp.MustCompile(`^(\d{4})-(.*)$`)

// stripReplyCode removes a leading "NNNN-" reply code from a line while
// keeping any indentation that follows it.
func stripReplyCode(line string) string {
	m := replyCodeRegex.FindStringSubmatch(line)
	if m == nil {
		return line
	}

	return m[2]
}

// indentLevel counts the indentation level of a line. BIRD indents with tabs
// (one level per tab); interactive clients may render the same structure with
// eight spaces per level, so both are accepted.
func indentLevel(line string) int {
	tabs := 0
	for tabs < len(line) && line[tabs] == '\t' {
		tabs++
	}

	if tabs > 0 {
		return tabs
	}

	spaces := 0
	for spaces < len(line) && line[spaces] == ' ' {
		spaces++
	}

	return spaces / 8
}

// trimmedLine returns a line without reply code and without surrounding
// whitespace.
func trimmedLine(line string) string {
	return strings.TrimSpace(stripReplyCode(line))
}
