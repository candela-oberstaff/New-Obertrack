package utils

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/vanng822/go-premailer/premailer"
)

// RenderBlocksToHTML converts the JSON blocks array stored in the template
// into a complete, inline-styled HTML email body ready for sending.
// backendURL is used to make relative image upload URLs absolute (e.g. /api/uploads/... → https://api.example.com/api/uploads/...).
func RenderBlocksToHTML(blocksJSON string, backendURL string) (string, error) {
	if blocksJSON == "" || blocksJSON == "[]" {
		return "<p>Sin contenido</p>", nil
	}

	var blocks []map[string]interface{}
	if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
		return "", fmt.Errorf("invalid blocks JSON: %w", err)
	}

	// 1. Detectar si hay un bloque de código crudo (raw) o bloque html
	var rawBlock map[string]interface{}
	for _, b := range blocks {
		t, _ := b["type"].(string)
		if t == "html" {
			rawBlock = b
			break
		}
		if t == "text" {
			if style, ok := b["style"].(map[string]interface{}); ok {
				if r, _ := style["raw"].(string); r == "true" {
					rawBlock = b
					break
				} else if rBool, ok := style["raw"].(bool); ok && rBool {
					rawBlock = b
					break
				}
			}
		}
	}

	// 4. DICCIONARIO LIMPIO DE TAILWIND Y ESTILOS RESPONSIVOS PARA GMAIL
	tailwindCSS := `.flex{display:flex}.flex-col{flex-direction:column}.items-center{align-items:center}.justify-between{justify-content:space-between}.grid{display:grid}.grid-cols-2{grid-template-columns:repeat(2,minmax(0,1fr))}.gap-2{gap:8px}.gap-4{gap:16px}.p-2{padding:8px}.p-3{padding:12px}.p-4{padding:16px}.p-6{padding:24px}.px-2{padding-left:8px;padding-right:8px}.px-4{padding-left:16px;padding-right:16px}.py-0.5{padding-top:2px;padding-bottom:2px}.py-2{padding-top:8px;padding-bottom:8px}.mb-2{margin-bottom:8px}.mb-4{margin-bottom:16px}.w-full{width:100%}.text-xs{font-size:12px}.text-sm{font-size:14px}.text-base{font-size:16px}.text-lg{font-size:18px}.font-normal{font-weight:400}.font-medium{font-weight:500}.font-semibold{font-weight:600}.font-bold{font-weight:700}.text-center{text-align:center}.text-left{text-align:left}.text-white{color:#ffffff}.text-gray-500{color:#6b7280}.text-gray-700{color:#374151}.text-gray-900{color:#111827}.rounded{border-radius:4px}.rounded-md{border-radius:6px}.rounded-lg{border-radius:8px}.border{border:1px solid #e5e7eb}.border-gray-200{border-color:#e5e7eb}.border-gray-400{border-color:#9ca3af}.bg-yellow-50{background-color:#fefce8}.bg-amber-50{background-color:#fffbeb}.border-yellow-200{border-color:#fef08a}.border-amber-200{border-color:#fde68a}.border-yellow-400{border-color:#facc15}.text-yellow-800{color:#854d0e}.text-amber-800{color:#92400e}.bg-purple-600{background-color:#8b5cf6}.bg-indigo-600{background-color:#4f46e5}`

	if rawBlock != nil {
		htmlCompleto, _ := rawBlock["content"].(string)
		
		// Reemplazar URLs relativas si es necesario
		if backendURL != "" {
			htmlCompleto = strings.ReplaceAll(htmlCompleto, "/api/uploads/", strings.TrimRight(backendURL, "/")+"/api/public/uploads/")
		}

		styleTag := fmt.Sprintf("<style>%s</style>", tailwindCSS)

		var htmlConEstilos string
		htmlLower := strings.ToLower(htmlCompleto)
		if idx := strings.Index(htmlLower, "<head>"); idx != -1 {
			htmlConEstilos = htmlCompleto[:idx+6] + styleTag + htmlCompleto[idx+6:]
		} else if idx := strings.Index(htmlLower, "<body>"); idx != -1 {
			htmlConEstilos = htmlCompleto[:idx+6] + styleTag + htmlCompleto[idx+6:]
		} else {
			htmlConEstilos = styleTag + htmlCompleto
		}

		// Limpiar selector universal * para evitar que Premailer rompa la especificidad de paddings/margins
		starSelectorRegex := regexp.MustCompile(`(?i)\*[^\{]*\{[^\}]*\}`)
		htmlConEstilosClean := starSelectorRegex.ReplaceAllString(htmlConEstilos, "")

		prem, err := premailer.NewPremailerFromString(htmlConEstilosClean, premailer.NewOptions())
		if err == nil {
			if inlined, err := prem.Transform(); err == nil {
				htmlCompleto = inlined
			}
		}

		// Re-inyección final para Gmail
		htmlInlinedLower := strings.ToLower(htmlCompleto)
		var htmlFinal string
		if idx := strings.Index(htmlInlinedLower, "</head>"); idx != -1 {
			htmlFinal = htmlCompleto[:idx] + styleTag + htmlCompleto[idx:]
		} else {
			htmlFinal = htmlCompleto
		}

		return htmlFinal, nil
	}

	// 2. Extraer los ajustes de la plantilla (settings block) para plantillas visuales
	var settingsBlock map[string]interface{}
	for _, b := range blocks {
		if t, _ := b["type"].(string); t == "settings" {
			settingsBlock = b
			break
		}
	}

	maxWidth := "600px"
	showHeader := true
	showFooter := true
	companyName := "Oberstaff"
	logoURL := "https://obertrack.com/logos/logo-oberstaff.png"
	headerBg := "#ffffff"
	footerBg := "#f8fafc"

	if settingsBlock != nil {
		if style, ok := settingsBlock["style"].(map[string]interface{}); ok {
			if mw, _ := style["maxWidth"].(string); mw != "" {
				maxWidth = mw
			}
			if sh, _ := style["showHeader"].(string); sh == "false" {
				showHeader = false
			}
			if sf, _ := style["showFooter"].(string); sf == "false" {
				showFooter = false
			}
			if cn, _ := style["companyName"].(string); cn != "" {
				companyName = cn
			}
			if lu, _ := style["logoUrl"].(string); lu != "" {
				logoURL = lu
			}
			if hb, _ := style["headerBg"].(string); hb != "" {
				headerBg = hb
			}
			if fb, _ := style["footerBg"].(string); fb != "" {
				footerBg = fb
			}
		}
	}

	bodyHTML := renderBlocks(blocks, backendURL)

	// 2. Construir cabecera y pie de página dinámicos
	headerHTML := ""
	if showHeader {
		headerHTML = fmt.Sprintf(`
			<tr>
				<td align="center" style="background-color:%s; padding:20px; text-align:center; border-bottom:1px solid #e2e8f0;">
					<a href="https://oberstaff.com" target="_blank" style="display:inline-block; text-decoration:none;">
						<img src="%s" alt="%s" style="max-height:50px; width:auto; border:0; display:block; margin:0 auto;" />
					</a>
				</td>
			</tr>`, headerBg, logoURL, companyName)
	}

	footerHTML := ""
	if showFooter {
		footerHTML = fmt.Sprintf(`
			<tr>
				<td align="center" style="background-color:%s; padding:16px; text-align:center; font-size:11px; color:#94a3b8; border-top:1px solid #e2e8f0; font-family:'Segoe UI',Helvetica,Arial,sans-serif;">
					&copy; 2026 %s · Este mensaje fue enviado automáticamente.
				</td>
			</tr>`, footerBg, companyName)
	}

	// 3. Envolvemos el contenido en una tabla limpia de email que respete el max-width de forma rígida para Gmail
	bgLight := "#f5f2fb"
	widthVal := strings.Replace(maxWidth, "px", "", 1)
	htmlCompleto := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Notificación</title>
</head>
<body style="font-family:'Segoe UI',Helvetica,Arial,sans-serif; background-color:%s; margin:0; padding:0; -webkit-font-smoothing:antialiased;">
    <div style="width:100%%; table-layout:fixed; background-color:%s; padding-top:40px; padding-bottom:40px;">
        <table class="email-container" align="center" border="0" cellpadding="0" cellspacing="0" width="%s" style="background-color:#ffffff; margin:0 auto; width:%s; max-width:%s; border-spacing:0; border-collapse:collapse; border-radius:12px; overflow:hidden; box-shadow:0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -1px rgba(0,0,0,0.06); border:1px solid #e2e8f0;">
            %s
            <tr>
                <td style="padding:0; margin:0; line-height:1.6; font-size:16px; vertical-align:top;">
                    %s
                </td>
            </tr>
            %s
        </table>
    </div>
</body>
</html>`, bgLight, bgLight, widthVal, maxWidth, maxWidth, headerHTML, bodyHTML, footerHTML)

	// 4. DICCIONARIO LIMPIO DE TAILWIND Y ESTILOS RESPONSIVOS PARA GMAIL
	responsiveCSS := fmt.Sprintf(`
		@media only screen and (max-width: %s) {
			.email-container {
				width: 100%% !important;
				max-width: 100%% !important;
			}
		}
	`, maxWidth)

	tailwindCSS = `.flex{display:flex}.flex-col{flex-direction:column}.items-center{align-items:center}.justify-between{justify-content:space-between}.grid{display:grid}.grid-cols-2{grid-template-columns:repeat(2,minmax(0,1fr))}.gap-2{gap:8px}.gap-4{gap:16px}.p-2{padding:8px}.p-3{padding:12px}.p-4{padding:16px}.p-6{padding:24px}.px-2{padding-left:8px;padding-right:8px}.px-4{padding-left:16px;padding-right:16px}.py-0.5{padding-top:2px;padding-bottom:2px}.py-2{padding-top:8px;padding-bottom:8px}.mb-2{margin-bottom:8px}.mb-4{margin-bottom:16px}.w-full{width:100%}.text-xs{font-size:12px}.text-sm{font-size:14px}.text-base{font-size:16px}.text-lg{font-size:18px}.font-normal{font-weight:400}.font-medium{font-weight:500}.font-semibold{font-weight:600}.font-bold{font-weight:700}.text-center{text-align:center}.text-left{text-align:left}.text-white{color:#ffffff}.text-gray-500{color:#6b7280}.text-gray-700{color:#374151}.text-gray-900{color:#111827}.rounded{border-radius:4px}.rounded-md{border-radius:6px}.rounded-lg{border-radius:8px}.border{border:1px solid #e5e7eb}.border-gray-200{border-color:#e5e7eb}.border-gray-400{border-color:#9ca3af}.bg-yellow-50{background-color:#fefce8}.bg-amber-50{background-color:#fffbeb}.border-yellow-200{border-color:#fef08a}.border-amber-200{border-color:#fde68a}.border-yellow-400{border-color:#facc15}.text-yellow-800{color:#854d0e}.text-amber-800{color:#92400e}.bg-purple-600{background-color:#8b5cf6}.bg-indigo-600{background-color:#4f46e5}`

	styleTag := fmt.Sprintf("<style>%s\n%s</style>", tailwindCSS, responsiveCSS)
	
	var htmlConEstilos string
	htmlLower := strings.ToLower(htmlCompleto)
	if idx := strings.Index(htmlLower, "<head>"); idx != -1 {
		htmlConEstilos = htmlCompleto[:idx+6] + styleTag + htmlCompleto[idx+6:]
	} else if idx := strings.Index(htmlLower, "<body>"); idx != -1 {
		htmlConEstilos = htmlCompleto[:idx+6] + styleTag + htmlCompleto[idx+6:]
	} else {
		htmlConEstilos = styleTag + htmlCompleto
	}

	// Limpiar selector universal * para evitar que Premailer rompa la especificidad de paddings/margins
	starSelectorRegex := regexp.MustCompile(`(?i)\*[^\{]*\{[^\}]*\}`)
	htmlConEstilosClean := starSelectorRegex.ReplaceAllString(htmlConEstilos, "")

	prem, err := premailer.NewPremailerFromString(htmlConEstilosClean, premailer.NewOptions())
	if err != nil {
		fmt.Printf("Error inicializando premailer: %v\n", err)
		return htmlConEstilosClean, nil
	}

	htmlInlined, err := prem.Transform()
	if err != nil {
		fmt.Printf("Error aplicando Inline CSS con Premailer: %v\n", err)
		return htmlConEstilos, nil
	}

	htmlInlinedLower := strings.ToLower(htmlInlined)
	var htmlFinal string
	if idx := strings.Index(htmlInlinedLower, "</head>"); idx != -1 {
		htmlFinal = htmlInlined[:idx] + styleTag + htmlInlined[idx:]
	} else {
		htmlFinal = htmlInlined
	}

	return htmlFinal, nil
}

func renderBlocks(blocks []map[string]interface{}, backendURL string) string {
	var sb strings.Builder

	for _, block := range blocks {
		blockType, _ := block["type"].(string)
		if blockType == "settings" {
			continue
		}
		content := block["content"]
		styleMap, _ := block["style"].(map[string]interface{})

		classStr, _ := block["class"].(string)
		if classStr == "" {
			classStr, _ = styleMap["className"].(string)
		}

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
			if fontSize == "" { fontSize = "16px" }
			if color == "" { color = "#1e293b" }
			
			sb.WriteString(fmt.Sprintf(
				`<table width="100%%" cellpadding="0" cellspacing="0" style="width:100%%; border-collapse:collapse;">
					<tr>
						<td class="%s" style="%s font-size:%s; color:%s; line-height:1.6; font-family:'Segoe UI',Helvetica,Arial,sans-serif;">%s</td>
					</tr>
				 </table>`,
				classStr, cellStyle, fontSize, color, text,
			))

		case "button":
			text, _ := content.(string)
			btnColor, _ := styleMap["backgroundColor"].(string)
			btnText, _ := styleMap["color"].(string)
			btnRadius, _ := styleMap["borderRadius"].(string)
			linkURL, _ := styleMap["linkUrl"].(string)
			align, _ := styleMap["align"].(string)
			padding, _ := styleMap["padding"].(string)

			if btnColor == "" { btnColor = "#cc33cc" }
			if btnText == "" { btnText = "#ffffff" }
			if btnRadius == "" { btnRadius = "8px" }
			if linkURL == "" { linkURL = "#" }
			if align == "" { align = "center" }
			if padding == "" { padding = "12px 28px" }

			sb.WriteString(fmt.Sprintf(
				`<table width="100%%" cellpadding="0" cellspacing="0" style="width:100%%; border-collapse:collapse;">
					<tr>
						<td align="%s" style="%s text-align:%s;">
							<a href="%s" class="%s" style="display:inline-block; padding:%s; background:%s; color:%s; border-radius:%s; text-decoration:none; font-weight:600; font-size:15px;">%s</a>
						</td>
					</tr>
				 </table>`,
				align, cellStyle, align, linkURL, classStr, padding, btnColor, btnText, btnRadius, escapeHTML(text),
			))

		case "html":
			htmlContent, _ := content.(string)
			sb.WriteString(htmlContent)

		case "image":
			src, _ := content.(string)
			width, _ := styleMap["width"].(string)
			if width == "" { width = "100%" }
			if backendURL != "" && strings.HasPrefix(src, "/api/uploads/") {
				src = strings.TrimRight(backendURL, "/") + strings.Replace(src, "/api/uploads/", "/api/public/uploads/", 1)
			}
			
			sb.WriteString(fmt.Sprintf(
				`<table width="100%%" cellpadding="0" cellspacing="0" style="width:100%%; border-collapse:collapse;">
					<tr>
						<td style="%s">
							<img src="%s" width="%s" class="%s" style="width:%s; max-width:100%%; height:auto; display:block; border-radius:4px; border:0;" alt=""/>
						</td>
					</tr>
				 </table>`,
				cellStyle, src, width, classStr, width,
			))

		case "divider":
			borderHeight, _ := styleMap["borderHeight"].(string)
			if borderHeight == "" { borderHeight = "1px" }
			borderColor, _ := styleMap["borderColor"].(string)
			if borderColor == "" { borderColor = "#e2e8f0" }
			borderStyle, _ := styleMap["borderStyle"].(string)
			if borderStyle == "" { borderStyle = "solid" }
			
			sb.WriteString(fmt.Sprintf(
				`<table width="100%%" cellpadding="0" cellspacing="0" style="width:100%%; border-collapse:collapse;">
					<tr>
						<td style="padding:8px 24px;">
							<div class="%s" style="border-top:%s %s %s; height:0; line-height:0; font-size:0;">&nbsp;</div>
						</td>
					</tr>
				 </table>`,
				classStr, borderHeight, borderStyle, borderColor,
			))

		case "spacer":
			height, _ := styleMap["height"].(string)
			if height == "" { height = "24px" }
			sb.WriteString(fmt.Sprintf(
				`<table width="100%%" cellpadding="0" cellspacing="0" style="width:100%%; border-collapse:collapse;">
					<tr><td height="%s" style="font-size:0; line-height:0; height:%s;">&nbsp;</td></tr>
				 </table>`, 
				height, height,
			))

		case "columns":
			cols, _ := content.([]interface{})
			var colsHTML []string
			for _, cVal := range cols {
				cMap, _ := cVal.(map[string]interface{})
				width, _ := cMap["width"].(string)
				if width == "" { width = "50%" }
				subBlocksRaw, _ := cMap["blocks"].([]interface{})
				var subBlocks []map[string]interface{}
				for _, sbRaw := range subBlocksRaw {
					if sbMap, ok := sbRaw.(map[string]interface{}); ok {
						subBlocks = append(subBlocks, sbMap)
					}
				}
				colsHTML = append(colsHTML, fmt.Sprintf(
					`<td width="%s" valign="top" style="width:%s; vertical-align:top;">%s</td>`,
					width, width, renderBlocks(subBlocks, backendURL),
				))
			}
			sb.WriteString(fmt.Sprintf(
				`<table width="100%%" cellpadding="0" cellspacing="0" style="width:100%%; border-collapse:collapse; table-layout:fixed;">
					<tr>
						<td style="%s">
							<table width="100%%" cellpadding="0" cellspacing="0" style="width:100%%; border-collapse:collapse; table-layout:fixed;">
								<tr>%s</tr>
							</table>
						</td>
					</tr>
				 </table>`,
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
			
			var links []string
			for net, data := range socialData {
				dataMap, _ := data.(map[string]interface{})
				active, _ := dataMap["active"].(bool)
				if !active { continue }
				url, _ := dataMap["url"].(string)
				if url == "" { url = iconLinks[net] }
				
				links = append(links, fmt.Sprintf(
					`<a href="%s" style="display:inline-block; margin:0 8px; color:#64748b; font-size:13px; text-decoration:none; font-family:'Segoe UI',Arial,sans-serif;">%s</a>`,
					url, strings.Title(net),
				))
			}
			
			sb.WriteString(fmt.Sprintf(
				`<table width="100%%" cellpadding="0" cellspacing="0" style="width:100%%; border-collapse:collapse;">
					<tr>
						<td align="center" style="%s text-align:center;">%s</td>
					</tr>
				 </table>`,
				cellStyle, strings.Join(links, ""),
			))
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