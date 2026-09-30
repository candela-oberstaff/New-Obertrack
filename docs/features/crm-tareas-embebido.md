# Tareas embebida en el CRM: implementación en Obertrack

El contrato está en [../integracion-crm-tareas.md](../integracion-crm-tareas.md) y
tiene que coincidir **palabra por palabra** con `src/lib/obertrack/CONTRATO-SSO.md`
del CRM (§11). Este archivo es lo de nuestro lado: dónde está cada cosa y las
decisiones que el contrato no fija.

## Dónde vive cada cosa

| Pieza | Archivo |
|---|---|
| Canje: firma, claims, jti, empresa, pertenencia | `backend/internal/service/crm_sso_service.go` |
| `GET /api/auth/crm` (redirecciones 303, auditoría) | `backend/internal/handlers/crm_sso.go` |
| jti consumidos | tabla `crm_login_nonces`, migración `202609291200_crm_login_nonces` |
| Sesión acotada (solo access, tenant forzado) | `IssueScopedAccess` en `backend/internal/service/auth_token_service.go` |
| Claim `scope` y su middleware | `backend/internal/middleware/auth.go`, `backend/internal/middleware/scope.go` |
| Lista de rutas del alcance `tasks` | `backend/internal/routes/scope_routes.go` |
| Encuadre, cabeceras y log enmascarado | `frontend/nginx.conf.template` |
| Validación de `CRM_ORIGIN` | `frontend/docker-entrypoint.d/18-crm-origin.envsh` |
| `/embed/config.js` en `npm run dev` | plugin `embedConfigDev` en `frontend/vite.config.ts` |
| Token en memoria, `ready`, `session_expired` | `frontend/src/embed/session.ts` |
| `postMessage` al CRM | `frontend/src/embed/crmBridge.ts` |
| Pantallas `/embed/tareas`, `/embed/horas` y `/embed/error` | `frontend/src/embed/EmbedView.tsx`, `EmbedError.tsx`, `EmbedMessage.tsx` |
| Adjuntos e imágenes con la cabecera | `frontend/src/embed/authedFiles.ts` |

## Decisiones de nuestro lado

- **El jti se consume antes de mirar empresa y usuario.** Un token válido que
  acaba en `user_not_found` tampoco se puede reintentar.
- **Correo:** se busca tal cual llega y, si no aparece, en minúsculas.
- **Fallo interno** (base de datos, firma de la sesión): 503 en JSON, no una
  redirección. Ningún código del §7 lo describe, y el tiempo límite de 20 s
  del CRM lo cubre.
- **Auditoría:** `auth.crm_login` / `auth.crm_login_failed`, con `EntityID` =
  jti y en `Changes` el código, el motivo real, el kid y la empresa. Nunca el
  token.
- **Pertenencia:** se rechazan superadmin (tipo o bandera `is_superadmin`) y
  Customer Success. Los analistas de IT con empleo activo pasan (v1.1, §8).
- **Lista del alcance:** todo lo que usa la vista de Tareas normal, incluida la
  gestión de tableros, fases y miembros (v1.1, §10). Fuera: las
  automatizaciones (la vista embebida no enseña ni el constructor ni su
  indicador; las puertas llegan en el 422 del propio PUT), el selector de
  empresa de superadmin, las notificaciones, el chat y los sockets.
- **Test de la lista:** `scope_routes_test.go` monta el router real y recorre
  todas las rutas protegidas. Una ruta nueva queda cerrada a las sesiones
  acotadas sin que nadie tenga que acordarse.

## Vista de Horas (en curso, contrato v1.3 pendiente de acordar)

- **Claim `scope`:** opcional en el token del CRM, con valores `"tasks"` o
  `"hours"`. Sin él se usa `"tasks"`, así que los tokens v1.2 siguen igual.
  Cualquier otro valor, incluidos el texto vacío, las mayúsculas o una lista,
  da `invalid_token`. El canje redirige a `/embed/tareas` o a `/embed/horas`, y
  la auditoría registra el alcance.
- **Lista de `hours`** (`scope_routes.go`):
  - `/auth/me`;
  - lectura, alta, edición, aprobación y rechazo de `/work-hours`;
  - `GET /users/employees`, el filtro de la cuenta de empresa;
  - las subidas de archivos del editor.
- **Fuera de `hours`:**
  - `send-report`, porque manda correos;
  - `report/pdf|excel`, porque la vista los genera en el navegador;
  - todo `/admin`, que es solo para superadmin y CS, y el canje los rechaza.
- **Test:** `scope_routes_test.go` recorre el router para cada alcance y
  comprueba que ninguno alcance las rutas del módulo del otro.
- **Frontend:** `EmbedView` sirve `/embed/tareas` y `/embed/horas` con la misma
  sesión en memoria, los mismos mensajes y el mismo manejo de archivos.
  `ready` lleva `{ company_id, scope }`, y el `scope` se lee del token.
- **En la vista de Horas embebida:**
  - no aparece «Enviar por Correo»;
  - refresca cada 30 s;
  - `ready` se envía al terminar de cargar los registros.
- **Impresión (también en Obertrack normal):** la ventana de impresión escribía
  las actividades y los nombres sin sanear. Ahora las actividades pasan por
  `sanitizeRichHtml` y el resto del texto se escapa. En el embebido, las
  imágenes de `/api/uploads` se pasan a `blob:` antes de escribir la ventana
  (`resolveUploadImages`). Para que `window.print()` funcione dentro del
  iframe, el CRM tiene que añadir `allow-modals` (v1.3, §5).
- **Log del backend:** `RedactedLogFormatter` (en `middleware/logformat.go`)
  sustituye el query de `/api/auth/crm` por `[redacted]` en el log de Gin.

## Encuadre (nginx)

- `nginx.conf.template` es una plantilla: el entrypoint de la imagen la pasa por
  envsubst al arrancar, con `NGINX_ENVSUBST_FILTER=^CRM_` para no tocar las
  `$variables` de nginx.
- **Una sola variable, en tiempo de ejecución: `CRM_ORIGIN`.** No hay
  `VITE_CRM_ORIGIN` en el build. `18-crm-origin.envsh` la valida y deriva:
  - `CRM_FRAME_ANCESTORS`: el `frame-ancestors` de `/api/auth/crm` y `/embed/`;
  - `CRM_EMBED_ORIGIN`: el `targetOrigin` que nginx escribe en `/embed/config.js`.
- Se acepta `https://host[:puerto]`, y `http://` solo para `localhost` o
  `127.0.0.1`. Una variable vacía o mal formada deja la vista cerrada:
  `frame-ancestors 'none'` y `crmOrigin: ""`, así que no se envía ningún
  `postMessage`. La validación también impide inyectar texto en la CSP o en el JS.
- Cada `location` del embebido repite las cabeceras de seguridad, porque un
  `add_header` propio anula las del `server`, y omite `X-Frame-Options`.
- `/embed/` sirve `index.html` con `rewrite ... break` y no con `try_files`: el
  fallback de `try_files` salta a `location /`, que responde con `DENY`.
- `/api/auth/crm` escribe en el access log con `$uri`, sin el query. El
  `error_log` de nginx sí incluiría la petición completa si el proxy fallara
  (backend caído). El token es de un solo uso y dura 60 s, así que se acepta.

## Frontend embebido

- **Arranque:** `bootEmbedSession()` corre en `main.tsx` antes de montar React.
  Si la ruta es `/embed/...`, activa el modo, lee `#s=`, limpia el hash y carga
  `/embed/config.js`. Tiene que ir antes porque, si no, `AuthProvider`
  preguntaría `/auth/me` sin cabecera: 401, refresh y redirección a `/login`.
- **Cliente HTTP:** en modo embebido va siempre con `Authorization: Bearer` y
  `withCredentials: false`. Un 401 no refresca ni manda a `/login`: emite
  `session_expired` (una sola vez) y la vista muestra la pantalla de sesión
  terminada. No navega a `/embed/error`, para no mandar además un `error`.
- **Mensajes:** objetos planos, como los lee el CRM (`{type, company_id}`,
  `{type, code}`). Se encolan hasta que llega `config.js`. `company_id` se lee
  del `tenant_id` del token, sin verificarlo: solo sirve para anunciarlo, y el
  CRM comprueba que coincida con la empresa que pidió.
- **`ready`** se envía cuando terminan de cargar los tableros, haya o no.
- **Sin Layout:** ni menú, ni campana, ni sockets, ni novedades. Tareas refresca
  cada 30 s con `refetchInterval`, que React Query pausa con la pestaña oculta.
- **Automatizaciones:** el botón no aparece en la vista embebida. Las rutas de
  `/workflows` salieron de la lista del alcance.
- **Archivos:** `/api/uploads/...` exige sesión, y un `<img>` o un `<a>` no
  llevan la cabecera. `authedFiles.ts` reescribe las imágenes a `blob:` y abre
  los enlaces como `blob:` en otra pestaña. El editor de texto enriquecido
  devuelve esas URLs a `/api/uploads/...` antes de guardar
  (`restoreUploadUrls`). Queda un 401 en consola por imagen, la primera vez que
  intenta cargar sin cabecera, antes de reescribirse.
- **`localStorage`:** Chrome con las cookies de terceros bloqueadas hace que
  tocarlo dentro del iframe lance una excepción, y Tareas lo usa para sus
  preferencias. En ese caso se sustituye por uno en memoria.
- **`access_suspended`:** el código no dice cuál de los rechazos del login fue,
  así que la pantalla usa un texto que cubre los tres.

## Variables

| Variable | Dónde | Qué es |
|---|---|---|
| `CRM_SSO_PUBLIC_KEYS` | backend | `{"<kid>": "<PEM>"}`. Sin ella, el canje responde 503 |
| `CRM_ORIGIN` | frontend (nginx) | Origen del CRM, sin ruta ni barra final. Tiene que ser idéntico al `FRONTEND_URL` del CRM |

## Probar en local

Con un par `crm-dev-<AAAA-MM>` (§10: no hay staging):

```
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out crm-dev.key
openssl pkey -in crm-dev.key -pubout -out crm-dev.pub
```

El `.pub` va en `CRM_SSO_PUBLIC_KEYS` del backend local y la privada en el CRM
local. La privada no pasa por git.
