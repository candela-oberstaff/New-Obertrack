package utils

import (
	"fmt"
	"os"
)

// WrapInPremiumTemplate takes raw HTML content and wraps it in a modern, premium design
// WITHOUT a header logo (since content templates already provide their own logo).
func WrapInPremiumTemplate(title string, content string) string {
	bgLight := "#f5f2fb"      // Lavender Mist
	textColor := "#060b23"    // Prussian Blue
	
	companyName := os.Getenv("COMPANY_NAME")
	if companyName == "" {
		companyName = "Oberstaff"
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
</head>
<body style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: %s; margin: 0; padding: 0; -webkit-font-smoothing: antialiased;">
    <div style="width: 100%%; table-layout: fixed; background-color: %s; padding-bottom: 40px;">
        <div style="height: 40px;"></div>
        <table align="center" width="100%%" style="background-color: #ffffff; margin: 0 auto; max-width: 600px; border-spacing: 0; color: %s; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);">
            <tr>
                <td style="padding: 40px 30px; line-height: 1.6; font-size: 16px;">
                    %s
                </td>
            </tr>
            <tr>
                <td style="padding: 20px; text-align: center; font-size: 12px; color: #64748b;">
                    &copy; %d %s. Todos los derechos reservados.
                </td>
            </tr>
        </table>
    </div>
</body>
</html>
	`, title, bgLight, bgLight, textColor, content, 2026, companyName)
}

