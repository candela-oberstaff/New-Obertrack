package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RenderBlocksToHTML converts the JSON blocks array stored in the template
// into a complete, inline-styled HTML email body ready for sending.
func RenderBlocksToHTML(blocksJSON string) (string, error) {
	if blocksJSON == "" || blocksJSON == "[]" {
		return "<p>Sin contenido</p>", nil
	}

	var blocks []map[string]interface{}
	if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
		return "", fmt.Errorf("invalid blocks JSON: %w", err)
	}

	var sb strings.Builder

	for _, block := range blocks {
		blockType, _ := block["type"].(string)
		content := block["content"]
		styleMap, _ := block["style"].(map[string]interface{})

		containerBg, _ := styleMap["containerBackground"].(string)
		cellStyle := "padding:16px 24px;"
		if containerBg != "" {
			cellStyle += fmt.Sprintf("background-color:%s;", containerBg)
		}

		switch blockType {
		case "text":
			text, _ := content.(string)
			fontSize, _ := styleMap["fontSize"].(string)
			color, _ := styleMap["color"].(string)
			if fontSize == "" {
				fontSize = "16px"
			}
			if color == "" {
				color = "#1e293b"
			}
			sb.WriteString(fmt.Sprintf(
				`<div style="%sfont-size:%s;color:%s;line-height:1.6;">%s</div>`,
				cellStyle, fontSize, color, escapeHTML(text),
			))

		case "button":
			text, _ := content.(string)
			btnColor, _ := styleMap["backgroundColor"].(string)
			btnText, _ := styleMap["color"].(string)
			btnRadius, _ := styleMap["borderRadius"].(string)
			linkURL, _ := styleMap["linkUrl"].(string)
			if btnColor == "" {
				btnColor = "#cc33cc" // Vivid Orchid as default
			}
			if btnText == "" {
				btnText = "#ffffff"
			}
			if btnRadius == "" {
				btnRadius = "8px"
			}
			if linkURL == "" {
				linkURL = "#"
			}
			sb.WriteString(fmt.Sprintf(
				`<div style="%stext-align:center;"><a href="%s" style="display:inline-block;padding:12px 28px;background:%s;color:%s;border-radius:%s;text-decoration:none;font-weight:600;font-size:15px;">%s</a></div>`,
				cellStyle, linkURL, btnColor, btnText, btnRadius, escapeHTML(text),
			))

		case "image":
			src, _ := content.(string)
			width, _ := styleMap["width"].(string)
			if width == "" {
				width = "100%"
			}
			sb.WriteString(fmt.Sprintf(
				`<div style="%s"><img src="%s" style="width:%s;max-width:100%%;height:auto;display:block;border-radius:4px;" alt=""/></div>`,
				cellStyle, src, width,
			))

		case "divider":
			sb.WriteString(`<div style="padding:8px 24px;"><hr style="border:none;border-top:1px solid #e2e8f0;margin:0;"/></div>`)

		case "spacer":
			sb.WriteString(`<div style="height:24px;"></div>`)

		case "social":
			socialData, _ := content.(map[string]interface{})
			iconLinks := map[string]string{
				"facebook":  "https://facebook.com",
				"instagram": "https://instagram.com",
				"twitter":   "https://twitter.com",
				"linkedin":  "https://linkedin.com",
				"youtube":   "https://youtube.com",
			}
			sb.WriteString(`<div style="` + cellStyle + `text-align:center;">`)
			for net, data := range socialData {
				dataMap, _ := data.(map[string]interface{})
				active, _ := dataMap["active"].(bool)
				if !active {
					continue
				}
				url, _ := dataMap["url"].(string)
				if url == "" {
					url = iconLinks[net]
				}
				sb.WriteString(fmt.Sprintf(
					`<a href="%s" style="display:inline-block;margin:0 8px;color:#64748b;font-size:13px;text-decoration:none;">%s</a>`,
					url, strings.Title(net),
				))
			}
			sb.WriteString(`</div>`)
		}
	}

	return WrapInPremiumTemplate("Notificación", sb.String()), nil
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
