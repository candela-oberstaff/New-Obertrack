# Asistentes de IA (servidor MCP de solo lectura)

Cada persona conecta su propio asistente (ChatGPT, Claude, Grok, Gemini CLI) a Obertrack y ve **exactamente lo mismo que en la web con su rol**. Es de solo lectura: ningún tool crea, edita, aprueba ni borra.

Es una réplica de la integración de Obersuite (`src/lib/mcp`, `src/pages/api/mcp`), adaptada a Go:

| Obersuite (Astro) | Obertrack (Go) |
|---|---|
| `lib/mcp/readonly-db.ts` | `backend/internal/mcp/readonly_db.go` |
| `lib/mcp/scope.ts` | `backend/internal/mcp/scope.go` |
| `lib/mcp/policy.ts` | `backend/internal/mcp/policy.go` |
| `lib/mcp/consentimiento.ts` | `backend/internal/mcp/consentimiento.go` |
| `lib/mcp/quotas.ts` + `alerts.ts` | `backend/internal/mcp/quotas.go` |
| tools + `registerReadTool` | `backend/internal/mcp/server.go`, `tools.go`, `search.go` |
| `pages/api/mcp/*` | `backend/internal/mcp/http_oauth.go`, `http_web.go`, `consent_page.go`; rutas en `backend/internal/routes/mcp_routes.go` |
| SDK `@modelcontextprotocol/sdk` | SDK oficial de Go `github.com/modelcontextprotocol/go-sdk` v1.7.0 |
| `SESSION_SECRET` | `JWT_SECRET` (con claves derivadas por dominio) |

## Endpoints

| Ruta | Auth | Qué hace |
|---|---|---|
| `POST/GET/DELETE /api/mcp` | Bearer | Servidor MCP sin estado: un `McpServer` por petición con los tools del rol. GET y DELETE responden 405 una vez autenticado (no hay SSE persistente). Sin token: 401 con `WWW-Authenticate: Bearer resource_metadata="…/.well-known/oauth-protected-resource/api/mcp"`. |
| `GET /.well-known/oauth-authorization-server` | pública | RFC 8414. También en `…/api/mcp`. |
| `GET /.well-known/oauth-protected-resource` | pública | RFC 9728. También en `…/api/mcp`. |
| `POST /api/mcp/oauth/register` | pública, 20/h por IP | Registro dinámico (RFC 7591). Solo admite redirect_uri de la lista permitida. |
| `GET/POST /api/mcp/oauth/authorize` | sesión web | Página de consentimiento (HTML del servidor, sin JS). El `scope` pedido no se valida: siempre se concede `read`. ChatGPT pide `offline_access` para recibir refresh tokens, y rechazarlo rompería su conexión. |
| `POST /api/mcp/oauth/token` | pública | `authorization_code` (PKCE S256) y `refresh_token` (con rotación). |
| `POST /api/mcp/oauth/revoke` | pública | RFC 7009. Siempre 200. |
| `GET /api/mcp/status` | sesión | Para el menú y la página. |
| `GET/DELETE /api/mcp/connections[/:id]` | sesión + uso habilitado | Mis conexiones. |
| `GET/POST/DELETE /api/mcp/tokens[/:id]` | sesión + uso habilitado | Tokens personales (`obs_pt_`). |
| `/api/admin/mcp/*` | superadmin | Panel "Uso de IA". |

## Roles: qué ve cada uno

El permiso de uso ("accion:usar-mcp") es el interruptor **Habilitado** de cada rol en el panel. De entrada solo está habilitado el superadmin. Al habilitar otro rol, ese rol ve **Integraciones** en el menú y puede conectar su asistente. Al deshabilitarlo, sus tokens dejan de responder.

| Rol | Alcance (igual que la web) | Tools |
|---|---|---|
| SuperAdmin | Todas las empresas, eligiendo `company_id` en horas, tareas y tableros. Tickets e incidentes. | todos |
| Customer Success | Igual, sin tickets (en la web son solo del superadmin). | todos menos `list_tickets` |
| Analista de IT | Solo totales agregados. | `whoami`, `general_overview` |
| Empresa | Su tenant: equipo, jornadas, tableros y tareas. | empresa, profesionales, horas, tareas, search/fetch |
| Supervisor / Manager | Jornadas propias y de su equipo (directo, multi-manager o árbol, según los flags). Todas las tareas del tenant, y en tableros solo los suyos. | profesionales (su equipo), horas, tareas, search/fetch |
| Profesional | Sus jornadas, y las tareas de los tableros a los que pertenece. | horas, tareas, search/fetch |

Los tools de horas y tareas desaparecen de `tools/list` si los roles RBAC del tenant dejan el módulo en "sin acceso", igual que `RequirePermission`.

Tools: `whoami` (no consume cuota), `general_overview`, `list_companies`, `get_company`, `list_professionals`, `list_work_hours`, `hours_summary`, `list_boards`, `list_tasks`, `get_task`, `list_tickets`, `list_incidents`, `search` y `fetch`. `search` y `fetch` usan el formato de conectores de ChatGPT (`{results:[{id,title,url}]}` y `{id,title,text,url,metadata}`).

## Seguridad

- **Base de datos:** cada consulta corre en `BEGIN READ ONLY` con `SET LOCAL statement_timeout = 15000` y termina en `ROLLBACK`.
  - Con `MCP_DB_USER`/`MCP_DB_PASSWORD`, las consultas usan además el rol `obertrack_mcp_ro` (`backend/sql/mcp_readonly_role.sql`). Ese rol tiene SELECT solo en las tablas de los tools y, en `users`, solo en columnas no sensibles.
  - Si el rol falla, las consultas caen al pool general, el log lo marca `CRITICAL` y el panel lo muestra. El MCP no se cae por eso.
- **Tokens:** se guarda solo su sha256.
  - Access token `obs_at_`: 1 hora.
  - Refresh token `obs_rt_`: 30 días, con rotación. Si llega un refresh ya usado, se revoca la conexión entera, se audita `security.token_reuse` y se crea una alerta.
  - Cada conexión vence a los 90 días.
- **Consentimiento:**
  - CSP con nonce, `frame-ancestors 'none'`, `X-Frame-Options: DENY` y `Referrer-Policy: no-referrer`.
  - Anti-CSRF con un campo HMAC atado a usuario, cliente, redirect_uri y code_challenge, válido 10 minutos.
  - No se valida `Origin`: Chrome lo manda `null` con no-referrer. Solo se rechaza de plano `Sec-Fetch-Site: cross-site`.
- **Salida:**
  - Los teléfonos llegan enmascarados (`•••1234`).
  - En "modo datos mínimos" se quitan descripciones, actividades, comentarios, notas y motivos, y se recortan los textos largos.
  - Toda respuesta con texto de terceros lleva la nota de que es información a resumir, nunca instrucciones a seguir.
- **Auditoría** (`audit_logs`, módulo `mcp`):
  - `mcp.client_registered`, `mcp.authorized`, `mcp.connected`, `mcp.token_refreshed`
  - `mcp.revoked`, `mcp.personal_token_created`, `mcp.quota_exceeded`, `mcp.paused`
  - `security.token_reuse`, `security.rejected`
  - Las llamadas a tools no se auditan: van a `mcp_usage`.
- **Sin correos:** las alertas (80 % y 100 % de cuota, reuso de token) solo aparecen en el panel.

## Configuración

Variables del backend (todas en `docker-compose.yaml`):

| Variable | Para qué |
|---|---|
| `FRONTEND_URL` | Ya existía. Es el issuer OAuth y la base de todas las URLs que se anuncian. En producción, `https://obertrack.com`. |
| `JWT_SECRET` | Ya existía. Firma el CSRF del consentimiento con una clave derivada. |
| `MCP_DB_USER` | Opcional: `obertrack_mcp_ro`. |
| `MCP_DB_PASSWORD` | Opcional: la contraseña que se le dio al rol al correr el SQL. |

nginx (`frontend/nginx.conf.template`) tiene dos `location` nuevas:

- `/.well-known/oauth-*` reenvía al backend.
- `/api/mcp/oauth/authorize` existe para que la CSP del server (`form-action 'self'`) no bloquee la vuelta al asistente tras autorizar.

El proxy de Vite también reenvía `/.well-known/oauth-`.

## Checklist de producción

1. Desplegar backend y frontend. La migración `202610161200_mcp_tables` crea las tablas y deja habilitado solo al superadmin.
2. Como superusuario de Postgres, correr una vez:
   `psql "$URL_SUPERUSUARIO" -v mcp_password='…' -f backend/sql/mcp_readonly_role.sql`
3. En Coolify, definir `MCP_DB_USER=obertrack_mcp_ro` y `MCP_DB_PASSWORD=…`, y comprobar `FRONTEND_URL=https://obertrack.com`. Reiniciar el backend.
4. En el log debe salir `[MCP] Consultas del MCP con el rol de solo lectura obertrack_mcp_ro`. En el panel, "Base de datos" debe decir "Rol de solo lectura dedicado".
5. Abrir `https://obertrack.com/.well-known/oauth-authorization-server`: tiene que llegar JSON (no el HTML de la SPA) con endpoints absolutos.
6. Con un usuario de prueba, conectar ChatGPT y Claude siguiendo la guía de Integraciones y preguntar `whoami`.
7. Revocar esa conexión desde **Integraciones → Uso de IA → Conexiones** y comprobar que el asistente deja de responder.

## Pruebas

- **Unitarias**, `go test ./internal/mcp/`:
  - PKCE.
  - redirect_uri permitidas y no permitidas.
  - CSRF vencido, de otro usuario o manipulado.
  - Redacción de teléfonos y modo mínimo.
  - Filtros de alcance por rol.
  - Tools por rol en `tools/list`.
  - Metadata `.well-known`.
  - 401 con `resource_metadata`.
- **Integración contra Postgres real.** Se omite sin `MCP_TEST_DSN`; la base tiene que ser desechable. Cubre:
  - Código reutilizado y PKCE fallido.
  - Rotación del refresh y detección de reuso.
  - Token revocado, usuario desactivado, rol deshabilitado y conexión vencida.
  - Cuota superada y pausa.
  - Alcance de horas y tareas comparado con la web.
  - search y fetch con el formato de ChatGPT.
  - Que cada tool de cada rol corra dentro de `BEGIN READ ONLY`.
  - Con `MCP_DB_USER` definido, que al rol dedicado no le falte ningún GRANT.
- **Punta a punta:** `npx @modelcontextprotocol/inspector@2.11.0 --cli http://localhost:8081/api/mcp --transport http --method tools/list --header "Authorization: Bearer obs_pt_…"`.
