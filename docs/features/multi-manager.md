# Multi‑manager por profesional — Documento de diseño

> Estado: **propuesta / diseño** (no implementado). Pasar de **1 manager por profesional** a **N managers por profesional, por empresa**.

## 0. Decisiones tomadas

| Decisión | Elección |
|---|---|
| ¿Quién aprueba con varios managers? | **Cualquiera** de sus managers (semántica OR). Se conserva separación de funciones (no aprueba sus propias horas). |
| ¿Manager principal? | **Sí**: un principal + adicionales. El principal alimenta expediente/CV/display/default y espeja `employments.manager_id`. |
| Al asignar (individual o masivo) | **Aditivo**: agrega al conjunto (con opción de quitar). El bulk **añade**, no reemplaza. |
| Alcance | **Por empresa** (cuelga del `employment`), coherente con la unificación per‑empresa ya hecha. |

---

## 1. Estado actual (1‑a‑1)

- `users.manager_id *uint` — manager global canónico (lo escribe toda asignación).
- `employments.manager_id *uint` — manager por empresa (fuente para el scope de horas tras la unificación).
- Lógica que asume *un* manager: aprobación de horas, `GetTeam` / `CountReportsByManager` / `GetReportsByManager`, guard de degradación, `AssignToManager`, `ReassignTeam`, `BulkAssignManager`, `ensureValidManager`.

El cuello de botella es estructural: **un puntero no representa N**. Se necesita una relación muchos‑a‑muchos.

---

## 2. Modelo de datos propuesto

Nueva tabla de unión, con grano **por empleo** (un profesional puede tener distinto set de managers en cada empresa):

```sql
CREATE TABLE employment_managers (
    id            BIGSERIAL PRIMARY KEY,
    employment_id BIGINT NOT NULL REFERENCES employments(id) ON DELETE CASCADE,
    manager_id    BIGINT NOT NULL REFERENCES users(id),
    is_primary    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    UNIQUE (employment_id, manager_id)            -- no duplicados
);
-- Un único principal por empleo (índice parcial):
CREATE UNIQUE INDEX uq_employment_primary_manager
    ON employment_managers (employment_id)
    WHERE is_primary = TRUE AND deleted_at IS NULL;
CREATE INDEX idx_employment_managers_manager ON employment_managers (manager_id);
```

`employments.manager_id` **se conserva** como espejo del **principal** (back‑compat y lecturas simples). La tabla guarda **todos** los managers (incluido el principal con `is_primary=true`).

**Por qué `employment_id` y no `(user_id, company_id)`:** ya tenemos `employments` como fuente per‑empresa; colgar de ahí evita duplicar la noción de empresa activa y reusa `GetActive(userID, companyID)`.

---

## 3. Semántica

- **Aprobar / rechazar horas de una jornada (empresa = `work_hours.tenant_id`):** permitido si `userID` está en el set de managers del `employment` (user, company) — y `wh.UserID != userID` (separación de funciones intacta).
- **Equipo de un manager M (en su empresa activa):** profesionales cuyo `employment` activo tiene una fila `(employment_id, M)`.
- **Principal:** se usa para expediente/CV/display y como default; espeja `employments.manager_id`.
- **Notificaciones de jornada:** por defecto a **todos** los managers del empleo (decisión abierta §9).
- **Guard de degradación/eliminación de un manager M:** bloquear si existe alguna fila `(_, M)` activa (M aún gestiona gente). Reutiliza el patrón actual contra la tabla.

---

## 4. Cambios de API

| Endpoint actual | Cambio |
|---|---|
| `POST /users/:id/assign-manager` (set único) | Pasa a **gestión de set**: `POST /users/:id/managers` (agrega), `DELETE /users/:id/managers/:managerId` (quita), `PUT /users/:id/managers/:managerId/primary` (marca principal). |
| `POST /admin/bulk-assign-manager` | **Aditivo**: agrega el manager seleccionado al set de cada profesional (no reemplaza). |
| `POST /users/:id/reassign-team` | Reasignar el equipo de M: quita M del set de sus profesionales y (opcional) agrega N. |
| `GET /admin/users/:id/reports` | Sin cambio de contrato; la query va contra la tabla. |
| *(nuevo)* `GET /users/:id/managers` | Lista los managers (con principal) de un profesional, para la UI multi‑select. |

---

## 5. Queries antes / después (las críticas)

**Scope de horas — `workhour_repository.go`** (filtros `manager_id` y `manager_or_user_id`):

```sql
-- ANTES (per‑empresa, 1 manager):
JOIN employments e ON e.user_id = work_hours.user_id
 AND e.company_id = work_hours.tenant_id AND e.status='active'
WHERE e.manager_id = ?

-- DESPUÉS (N managers):
JOIN employments e ON e.user_id = work_hours.user_id
 AND e.company_id = work_hours.tenant_id AND e.status='active'
JOIN employment_managers em ON em.employment_id = e.id AND em.deleted_at IS NULL
WHERE em.manager_id = ?
```

**Aprobar — `workhour_service.go` Approve/Reject** (`emp.ManagerID == userID` → existe vínculo):

```go
// ANTES: emp, _ := GetActive(wh.UserID, wh.TenantID); emp.ManagerID == userID
// DESPUÉS:
ok, _ := s.employmentRepo.IsManagerOf(wh.UserID, wh.TenantID, userID) // EXISTS en employment_managers
if wh.UserID != userID && ok { canApprove = true }
```

**Equipo / conteo — `GetTeam` / `CountReportsByManager` / `GetReportsByManager`:**

```sql
-- DESPUÉS: profesionales gestionados por X en su empresa activa
SELECT u.* FROM users u
JOIN employments e ON e.user_id = u.id AND e.status='active'
JOIN employment_managers em ON em.employment_id = e.id AND em.manager_id = ? AND em.deleted_at IS NULL
WHERE u.is_active = true;
```

---

## 6. Impacto en el código (mapa)

**Backend**
- `models/` — nuevo `EmploymentManager`; `migrations.go` (tabla + backfill).
- `repository/employment_repository.go` — `AddManager`, `RemoveManager`, `SetPrimary`, `ListManagers(employmentID)`, `IsManagerOf(userID, companyID, managerID)`, `CountActiveByManager` (→ tabla), `ReassignManager` (→ tabla).
- `repository/user_repository.go` — `CountReportsByManager` / `GetReportsByManager` → join.
- `service/workhour_service.go` — Approve/Reject/Update/notify → `IsManagerOf` / lista de managers.
- `service/user_service.go` — `AssignToManager` → add/remove; `ReassignTeam`; guards usan el conteo nuevo.
- `service/admin_service.go` — `BulkAssignManager` aditivo; guards.
- `service/employment_service.go` — `UpdateEmploymentManager` (principal + set); sync.
- `handlers/` + `routes/` — endpoints nuevos del set.

**Frontend**
- `AdminUserDetail.tsx` — el `<select>` de MANAGER pasa a **chips multi‑select** (agregar/quitar) + marcar principal; el de por‑empleo igual.
- `Admin.tsx` (bulk) — "Asignar" pasa a **"Agregar manager"** (aditivo); el texto y el resultado lo reflejan.
- `TeamPanel.tsx`, expediente/CV — mostrar principal (+ adicionales).
- `types` — `managers: {id,name,is_primary}[]` además de `manager_id` (principal).

---

## 7. Migración por fases (no rompe nada)

1. **Fase 0 — Tabla + backfill.** Crear `employment_managers`; backfill: cada `employments.manager_id` no nulo → 1 fila `is_primary=true`. Lecturas siguen usando el puntero. *Reversible: drop tabla.*
2. **Fase 1 — Dual‑write.** `AssignToManager`/`bulk`/`reassign`/admin escriben la tabla **y** mantienen `employments.manager_id` = principal. Lecturas aún por puntero. *Sin cambio visible.*
3. **Fase 2 — Switch de lecturas.** Aprobación, scope de horas, equipo y guards pasan a la tabla (semántica "cualquier manager"). Para single‑manager el resultado es idéntico. *Detrás de feature flag para A/B.*
4. **Fase 3 — UI multi‑manager.** Chips + principal + agregar/quitar; bulk aditivo.
5. **Fase 4 — (opcional) deprecar `employments.manager_id`** una vez todo lee la tabla (o dejarlo como principal denormalizado permanente, recomendado por simplicidad).

Cada fase es desplegable y reversible por separado.

---

## 8. Riesgos y mitigaciones

- **Ciclos** (A gestiona a B y B a A): con N managers es más fácil. Mitigación: al agregar manager M a profesional P, rechazar si P ya gestiona a M (chequeo de arista directa; ciclos largos son improbables porque la aprobación es de un nivel).
- **Auto‑manager:** mantener el guard `managerID != userID`.
- **Performance:** un JOIN extra; índices en `(manager_id)` y `(employment_id)`. Volúmenes pequeños, impacto nulo.
- **Duplicados:** `UNIQUE(employment_id, manager_id)`.
- **Doble principal:** índice parcial único.
- **Separación de funciones:** intacta (`wh.UserID != userID`).
- **Coherencia espejo:** `employments.manager_id` debe seguir al `is_primary` (mantener en la misma transacción).

---

## 9. Decisiones abiertas

1. **Notificaciones:** ¿a todos los managers o solo al principal? (default propuesto: todos.)
2. **Límite** de managers por profesional (¿tope, p.ej. 5?).
3. **Quién** puede gestionar el set: empleador/superadmin (igual que hoy).
4. **Reasignar equipo:** ¿quita solo a M o ofrece "mover a N"? (propuesto: quita M, opción de agregar N.)
5. **Vista de expediente/CV:** ¿muestra solo principal o lista completa?

---

## 10. Esfuerzo estimado (orientativo)

- Fase 0‑1 (tabla + backfill + dual‑write): **bajo‑medio**, backend acotado, reversible.
- Fase 2 (switch lecturas + flag): **medio**, toca workhour repo/service + guards; cubierto por tests (extender `manager_flow_test.go` al modelo de set).
- Fase 3 (UI multi‑select): **medio**, frontend (detalle, bulk, expediente).
- Fase 4 (cleanup): **bajo**.

Recomendación: empezar por **Fase 0‑1 detrás de flag** cuando se apruebe este diseño; el riesgo es mínimo y deja la base lista sin cambiar comportamiento.
