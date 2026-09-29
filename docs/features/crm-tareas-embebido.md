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
  gestión de tableros, fases y miembros (v1.1, §10). Fuera: el constructor de
  automatizaciones (solo se leen recetas y puertas), el selector de empresa de
  superadmin, las notificaciones, el chat y los sockets.
- **Test de la lista:** `scope_routes_test.go` monta el router real y recorre
  todas las rutas protegidas. Una ruta nueva queda cerrada a las sesiones
  acotadas sin que nadie tenga que acordarse.

## Variables

| Variable | Dónde | Qué es |
|---|---|---|
| `CRM_SSO_PUBLIC_KEYS` | backend | `{"<kid>": "<PEM>"}`. Sin ella, el canje responde 503 |

## Probar en local

Con un par `crm-dev-<AAAA-MM>` (§10: no hay staging):

```
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out crm-dev.key
openssl pkey -in crm-dev.key -pubout -out crm-dev.pub
```

El `.pub` va en `CRM_SSO_PUBLIC_KEYS` del backend local y la privada en el CRM
local. La privada no pasa por git.
