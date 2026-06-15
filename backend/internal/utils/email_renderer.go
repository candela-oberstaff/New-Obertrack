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

	bodyHTML := renderBlocks(blocks)
	return WrapInPremiumTemplate("Notificación", bodyHTML), nil
}

func renderBlocks(blocks []map[string]interface{}) string {
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
			borderWidth, _ := styleMap["borderWidth"].(string)
			borderColor, _ := styleMap["borderColor"].(string)
			borderStyle, _ := styleMap["borderStyle"].(string)
			align, _ := styleMap["align"].(string)
			padding, _ := styleMap["padding"].(string)

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
			if align == "" {
				align = "center"
			}
			if padding == "" {
				padding = "12px 28px"
			}

			borderCSS := ""
			if borderWidth != "" && borderColor != "" {
				bStyle := borderStyle
				if bStyle == "" {
					bStyle = "solid"
				}
				borderCSS = fmt.Sprintf("border:%s %s %s;", borderWidth, bStyle, borderColor)
			}

			sb.WriteString(fmt.Sprintf(
				`<div style="%stext-align:%s;"><a href="%s" style="display:inline-block;padding:%s;background:%s;color:%s;border-radius:%s;text-decoration:none;font-weight:600;font-size:15px;%s">%s</a></div>`,
				cellStyle, align, linkURL, padding, btnColor, btnText, btnRadius, borderCSS, escapeHTML(text),
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
			borderHeight, _ := styleMap["borderHeight"].(string)
			if borderHeight == "" {
				borderHeight = "1px"
			}
			borderColor, _ := styleMap["borderColor"].(string)
			if borderColor == "" {
				borderColor = "#e2e8f0"
			}
			borderStyle, _ := styleMap["borderStyle"].(string)
			if borderStyle == "" {
				borderStyle = "solid"
			}
			sb.WriteString(fmt.Sprintf(
				`<div style="padding:8px 24px;"><hr style="border:none;border-top:%s %s %s;margin:0;"/></div>`,
				borderHeight, borderStyle, borderColor,
			))

		case "spacer":
			height, _ := styleMap["height"].(string)
			if height == "" {
				height = "24px"
			}
			sb.WriteString(fmt.Sprintf(`<div style="height:%s;"></div>`, height))

		case "columns":
			cols, _ := content.([]interface{})
			var colsHTML []string
			for _, cVal := range cols {
				cMap, _ := cVal.(map[string]interface{})
				width, _ := cMap["width"].(string)
				if width == "" {
					width = "50%"
				}
				subBlocksRaw, _ := cMap["blocks"].([]interface{})
				var subBlocks []map[string]interface{}
				for _, sbRaw := range subBlocksRaw {
					if sbMap, ok := sbRaw.(map[string]interface{}); ok {
						subBlocks = append(subBlocks, sbMap)
					}
				}
				colsHTML = append(colsHTML, fmt.Sprintf(
					`<td width="%s" valign="top" style="width:%s;vertical-align:top;">%s</td>`,
					width, width, renderBlocks(subBlocks),
				))
			}
			sb.WriteString(fmt.Sprintf(
				`<div style="%s"><table width="100%%" cellpadding="0" cellspacing="0" style="width:100%%;border-collapse:collapse;table-layout:fixed;"><tr>%s</tr></table></div>`,
				cellStyle, strings.Join(colsHTML, ""),
			))

		case "social":
			socialData, _ := content.(map[string]interface{})
			iconLinks := map[string]string{
				"facebook":  "https://facebook.com",
				"instagram": "https://instagram.com",
				"twitter":   "https://twitter.com",
				"linkedin":  "https://linkedin.com",
				"youtube":   "https://youtube.com",
			}
			sb.WriteString(`<div style="`)
			sb.WriteString(cellStyle)
			sb.WriteString(`text-align:center;">`)
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

	return sb.String()
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
