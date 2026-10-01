# Contrato: acceso embebido a Tareas de Obertrack desde el CRM (Obersuite)

Versión 1.2 — cambios sobre 1.1: §1 actualizado (origen del CRM en tiempo de ejecución).

Cualquier cambio se acuerda aquí primero y sube la versión.

## 1. Partes y orígenes
- CRM (Obersuite): emite el token y enmarca el iframe. Origen: https://obersuite.oberstaff.com (+ staging, si existe).
- Obertrack: canjea el token y sirve la vista embebida. Origen: OBERTRACK_WEB_URL.
- Obertrack usa CRM_ORIGIN en tiempo de ejecución (nginx frame-ancestors y /embed/config.js para postMessage).
- El CRM usa OBERTRACK_WEB_URL (URL de canje y validación de event.origin).

## 2. Token
- JWT RS256. La clave privada vive solo en el CRM.
- Cabecera con kid obligatorio. Sin kid o con kid desconocido → invalid_token.
- Claims, todos obligatorios:
  - sub: string, correo del usuario leído de la BD del CRM.
  - obertrack_company_id: entero, users.id de la empresa EN OBERTRACK (el mismo id del padrón /integrations/obersuite/companies).
  - jti: UUID v4.
  - iss: "oberstaff-crm".
  - aud: "obertrack".
  - iat, exp: segundos Unix, exp = iat + 60.
- NO existe el claim empresa_id. No hay mapeo por obersuite_id ni tabla de enlaces.
- Obertrack rechaza exp - iat > 60 y aplica 30 s de leeway a exp, iat y nbf. Ambos servidores con NTP.
- jti de un solo uso: Obertrack lo consume con INSERT sobre restricción única.

## 3. Claves y rotación
- Obertrack lee CRM_SSO_PUBLIC_KEYS: JSON {"<kid>": "<PEM>", ...} al arrancar. Vacía o PEM inválido → fail-closed (503). Sin JWKS por ahora.
- Formato de kid: crm-<entorno>-<AAAA-MM>, por ejemplo crm-prod-2026-09.
- Rotación: par nuevo → Obertrack agrega la pública nueva junto a la vieja → el CRM cambia a la privada nueva → tras unos minutos Obertrack retira la vieja.
- Revocación: Obertrack elimina la pública.

## 4. Canje
GET <OBERTRACK_WEB_URL>/api/auth/crm?token=<jwt>
- El CRM arma la URL en su backend; el frontend solo la asigna al src del iframe.
- Éxito: 303 a /embed/tareas#s=<access_token>
  - Access de Obertrack con scope "tasks", 60 min, sin refresh y sin cookies.
  - El frontend lo guarda solo en memoria, limpia el hash con history.replaceState y lo manda en Authorization: Bearer.
- Error: 303 a /embed/error?code=<código> (sección 7).
- Cabeceras: Referrer-Policy: no-referrer, Cache-Control: no-store.
- /api/auth/crm fuera del access log de nginx o con el query enmascarado.

## 5. Encuadre
- Solo location = /api/auth/crm y location /embed/ se pueden enmarcar: sin X-Frame-Options, con frame-ancestors <CRM_ORIGIN>.
- El resto de Obertrack mantiene frame-ancestors 'none'.
- Ruta embebida por path (/embed/tareas), no por query.
- Iframe en el CRM: sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-downloads", referrerpolicy="no-referrer", title.

## 6. postMessage (solo iframe → CRM; el CRM no envía mensajes al iframe)
- obertrack:ready { company_id }: /auth/me respondió y el tablero se pintó.
- obertrack:error { code }: se cargó /embed/error.
- obertrack:session_expired {}: un 401 dentro de la sesión, O /embed/tareas se montó sin token en memoria (F5 dentro del iframe).
Reglas:
- Obertrack usa targetOrigin = CRM_ORIGIN, nunca "*".
- El CRM acepta solo event.origin === OBERTRACK_WEB_URL y event.source === iframe.contentWindow.
- Ningún mensaje contiene tokens.
- Ante session_expired, el CRM pide un token nuevo a su backend y remonta el iframe. Máximo 2 reintentos seguidos; luego muestra error con botón de reintento.
- Ante error, el CRM muestra el texto del código y no reintenta solo, salvo token_used, que trata como session_expired.
- El CRM decide el estado por los mensajes, no contando eventos load. El load solo sirve para un tiempo límite: sin mensajes en 20 s → error.

## 7. Códigos de error
- invalid_token: firma, kid, iss, aud o exp inválidos. "El enlace no es válido. Vuelve a abrir Tareas desde el CRM."
- token_used: jti ya consumido. Mismo texto.
- company_not_found: el id no existe, no es user_type=empresa o no está activa. "Esta empresa no está disponible en Obertrack. Contacta a soporte."
- user_not_found: el correo no existe o no pertenece a esa empresa (no se distingue a propósito). "Tu usuario no tiene acceso a Tareas de esta empresa."
- access_suspended: cuenta o empresa suspendida, inducción pendiente. Texto del login de Obertrack.
- session_expired: la sesión caducó estando dentro. "Tu sesión terminó. Vuelve a abrir Tareas desde el CRM."
La auditoría de Obertrack registra el motivo real y el kid. La del CRM registra el jti, nunca el token.

## 8. Pertenencia (la valida Obertrack)
- Cuenta de empresa: user.ID == obertrack_company_id.
- Manager o profesional: empleo activo en esa empresa.
- Superadmin y Customer Success: rechazados siempre (user_not_found).
- Mismas verificaciones del login: cuenta activa, inducción completada, empresa no suspendida.
- El tenant del token se fuerza a obertrack_company_id sin tocar users.empleador_id.
Esta regla es la barrera real: un enlace mal hecho en el CRM no debe dar acceso a otra empresa.
- Escenario vigente (A): obertrack_company_id es siempre la empresa Oberstaff en Obertrack, configurada por entorno en el CRM (OBERTRACK_COMPANY_ID). Los usuarios son vendedores del CRM con empleo activo en ella.
- Analistas de IT con empleo activo en Oberstaff: PERMITIDOS. El CRM ya filtra quién ve la pestaña por rol (ManagerVentas y EjecutivoVentas).

## 9. Alcance (scope "tasks")
- Lista permitida por c.FullPath(), fail-closed; lo demás → 403.
- Websockets bloqueados para tokens con alcance; refresco por intervalo (~30 s) mientras la vista está visible.
- authService.Refresh rechaza cualquier token con scope.
- La lista exacta vive en el código de Obertrack, con un test que recorre todas las rutas.

## 10. Decisiones
- Usuarios de la pestaña: escenario A (vendedores de Oberstaff, tablero interno de Oberstaff).
- El usuario embebido puede gestionar tableros, fases y miembros, igual que en la vista de Tareas normal.
- Staging: no existe; las pruebas se hacen en local con claves crm-dev.

## 11. Archivos
- El contrato vive en ambos repositorios con texto idéntico. Los detalles de implementación van en archivos aparte de cada proyecto.
