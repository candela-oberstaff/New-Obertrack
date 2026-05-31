package utils

import (
	"regexp"
	"strings"
)

func SanitizeHTML(input string) string {
	if input == "" {
		return ""
	}

	input = strings.TrimSpace(input)

	input = strings.ReplaceAll(input, "<script>", "")
	input = strings.ReplaceAll(input, "</script>", "")
	input = strings.ReplaceAll(input, "<script", "")
	input = strings.ReplaceAll(input, "javascript:", "")
	input = strings.ReplaceAll(input, "onerror=", "")
	input = strings.ReplaceAll(input, "onclick=", "")
	input = strings.ReplaceAll(input, "onload=", "")
	input = strings.ReplaceAll(input, "onmouseover=", "")
	input = strings.ReplaceAll(input, "onfocus=", "")
	input = strings.ReplaceAll(input, "onblur=", "")

	input = strings.ReplaceAll(input, "<iframe>", "")
	input = strings.ReplaceAll(input, "</iframe>", "")
	input = strings.ReplaceAll(input, "<object>", "")
	input = strings.ReplaceAll(input, "</object>", "")
	input = strings.ReplaceAll(input, "<embed>", "")
	input = strings.ReplaceAll(input, "</embed>", "")

	input = strings.ReplaceAll(input, "<style>", "")
	input = strings.ReplaceAll(input, "</style>", "")

	input = regexp.MustCompile(`\s+on\w+\s*=`).ReplaceAllString(input, " ")
	input = regexp.MustCompile(`\s+style\s*=`).ReplaceAllString(input, " ")

	return input
}
