package handlers

import "testing"

// safeReturnTo es la defensa contra redirect abierto: el destino post-consentimiento
// lo elige el frontend, viaja por la URL de Google y vuelve dentro del state, así
// que un enlace preparado podría llevar al usuario a un dominio ajeno justo
// después de autenticarse — el momento ideal para una pantalla de phishing.
func TestSafeReturnTo(t *testing.T) {
	valid := []string{
		"/perfil",
		"/perfil?tab=integraciones",
		"/empresa/ajustes",
		"/perfil#seccion",
	}
	for _, path := range valid {
		if got := safeReturnTo(path); got != path {
			t.Errorf("safeReturnTo(%q) = %q, debería conservarse", path, got)
		}
	}

	// Todo lo que pueda sacar al usuario del dominio cae al destino por defecto.
	hostile := []string{
		"https://evil.example/phishing",
		"http://evil.example",
		"//evil.example",             // protocolo-relativo: el navegador lo trata como absoluto
		"///evil.example",            // idem con barras extra
		"javascript:alert(1)",        // esquema ejecutable
		"data:text/html,<script>",    //
		"perfil",                     // relativo sin barra: se concatenaría mal
		"",                           // vacío
		"   ",                        // solo espacios
		"\\\\evil.example",           // barras invertidas (algunos navegadores las normalizan)
		"https://app.obertrack.test", // incluso el propio dominio: se espera una RUTA
	}
	for _, path := range hostile {
		if got := safeReturnTo(path); got != defaultGoogleReturnTo {
			t.Errorf("safeReturnTo(%q) = %q, se esperaba el destino por defecto %q", path, got, defaultGoogleReturnTo)
		}
	}
}
