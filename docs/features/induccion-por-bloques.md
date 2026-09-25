# Inducción por bloques — Documento de diseño

> Estado: **implementado** (25-sep-2026, rama `osvell-dev`). Pasar de **un video + un cuestionario** a **programas de inducción formados por bloques reutilizables**, cada bloque con su propio video y su propio cuestionario.
>
> Desvíos respecto al diseño original, anotados en la sección 10.

## 0. Decisiones tomadas (25-sep-2026)

| Decisión | Elección |
|---|---|
| ¿Un programa global o varios? | **Varios desde el inicio.** Cada programa se asigna a empresas; uno es el programa por defecto para quien no tenga asignación. |
| ¿De dónde sale el video de cada bloque? | **De Novedades**, como hoy. El bloque referencia un `tutorial_id`. |
| ¿Mínimo aprobatorio? | **Por bloque.** El programa define un valor por defecto que cada bloque puede sobrescribir. |
| ¿Cómo se avanza? | **Secuencial**: no se abre el bloque N+1 sin aprobar el N. No hay promedio global. |
| ¿Intentos? | **Por bloque.** Agotar los intentos de cualquier bloque bloquea al profesional y abre la alerta en Soporte indicando el bloque. |
| ¿Reanudable? | **Sí.** El progreso se guarda por bloque; el mismo enlace devuelve a la persona al bloque donde iba. |
| ¿Reinicio desde Soporte? | Reinicia el **programa completo** (como hoy). Reiniciar un solo bloque queda fuera de esta versión. |
| ¿Plantillas? | Todo bloque vive en una **biblioteca** y es reutilizable en cualquier programa. No hay un paso aparte de "guardar como plantilla". |

---

## 1. Estado actual

- `induction_configs` es una fila única: `tutorial_id`, `survey_id`, `passing_score`, `max_attempts`, `invite_ttl_days`, `is_active`.
- `induction_invites` congela al invitar: `tutorial_id`, `survey_id`, `passing_score`, `max_attempts`, `attempts`, `best_score`.
- `induction_attempts` es un intento del único cuestionario.
- La landing (`/induccion/:token`) es un wizard de tres pasos: video → cuestionario → resultado.
- El cuestionario es una `Survey` con `kind = 'induction'`, que la pestaña de Inducción arma en línea (preguntas con `correct_answer` y `weight`).

El cuello de botella es estructural: **una fila no representa una secuencia**.

---

## 2. Modelo de datos propuesto

```sql
-- Biblioteca de bloques (reutilizables)
CREATE TABLE induction_blocks (
  id            SERIAL PRIMARY KEY,
  name          VARCHAR(160) NOT NULL,
  description   TEXT,
  tutorial_id   INT NULL REFERENCES tutorials(id),   -- video (Novedades), opcional
  survey_id     INT NOT NULL REFERENCES surveys(id), -- cuestionario kind='induction'
  passing_score INT NULL,                            -- NULL = usa el del programa
  created_by    INT NOT NULL,
  created_at, updated_at, deleted_at
);

-- Programas
CREATE TABLE induction_programs (
  id                    SERIAL PRIMARY KEY,
  name                  VARCHAR(160) NOT NULL,
  description           TEXT,
  default_passing_score INT NOT NULL DEFAULT 70,
  max_attempts          INT NOT NULL DEFAULT 3,      -- por bloque
  is_default            BOOL NOT NULL DEFAULT false, -- uno solo en true (índice parcial único)
  is_active             BOOL NOT NULL DEFAULT true,
  created_at, updated_at, deleted_at
);

-- Orden de bloques dentro del programa
CREATE TABLE induction_program_blocks (
  program_id  INT NOT NULL REFERENCES induction_programs(id),
  block_id    INT NOT NULL REFERENCES induction_blocks(id),
  order_index INT NOT NULL,
  PRIMARY KEY (program_id, block_id)
);

-- Asignación programa → empresa
CREATE TABLE induction_program_companies (
  program_id INT NOT NULL REFERENCES induction_programs(id),
  company_id INT NOT NULL REFERENCES users(id),
  PRIMARY KEY (company_id)      -- una empresa tiene a lo sumo un programa
);

-- Progreso por bloque de una invitación
CREATE TABLE induction_block_progress (
  invite_id    INT NOT NULL REFERENCES induction_invites(id),
  block_id     INT NOT NULL,
  order_index  INT NOT NULL,
  status       VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending | passed | blocked
  attempts     INT NOT NULL DEFAULT 0,
  best_score   FLOAT NOT NULL DEFAULT 0,
  completed_at TIMESTAMP NULL,
  PRIMARY KEY (invite_id, block_id)
);
```

Cambios sobre tablas existentes:

| Tabla | Cambio |
|---|---|
| `induction_configs` | Queda como **interruptor global** y vigencia del enlace: `is_active`, `invite_ttl_days`. Se retiran `tutorial_id`, `survey_id`, `passing_score`, `max_attempts` (pasan al programa). |
| `induction_invites` | Se agrega `program_id` y `blocks_json` (snapshot: bloques, orden, mínimo y tutorial/survey de cada uno). Se conserva la regla de **congelado al invitar**: editar un programa no altera una invitación emitida. `attempts`, `best_score`, `tutorial_id` y `survey_id` dejan de usarse (se mantienen para no romper filas viejas). |
| `induction_attempts` | Se agrega `block_id`. |

`Ready()` pasa a ser: interruptor global encendido **y** existe un programa por defecto activo con al menos un bloque cuyo cuestionario existe.

### Migración de datos

1. Crear un bloque "Inducción general" con el `tutorial_id`, `survey_id` y `passing_score` de la fila actual.
2. Crear el programa "Programa por defecto" (`is_default = true`) con ese bloque y el `max_attempts` actual.
3. A cada invitación **pendiente** escribirle `program_id` y un `blocks_json` de un solo bloque, y crear su fila de `induction_block_progress` copiando `attempts` y `best_score`. Las aprobadas y bloqueadas se dejan como están.
4. A los intentos existentes asignarles el `block_id` del bloque migrado.

Nada de lo que está en curso se rompe: quien iba a mitad de la inducción sigue con el mismo enlace.

---

## 3. Resolución del programa al invitar

```
programa = asignado a la empresa contratante (req.CompanyID / user.EmpleadorID)
        ?? programa por defecto
```

- Desde el puente de Obersuite (`/hire`) siempre se resuelve así. El contrato de `/hire` **no cambia**: `induction_pending` sigue siendo un booleano.
- Desde Soporte (botón "Enviar inducción" en el expediente) se muestra un selector de programa, precargado con el resuelto. Permite mandar a alguien un programa distinto al de su empresa sin tocar la asignación.
- Si la empresa no tiene programa y no hay programa por defecto, la inducción cuenta como **no lista** y el alta sigue el flujo directo de siempre (misma regla que hoy con la config incompleta).

---

## 4. Flujo del profesional (landing)

```
[Bloque 1] video → cuestionario → resultado del bloque
    aprobó → [Bloque 2] ...
    falló con intentos → repite cuestionario del bloque 1
    agotó intentos → BLOQUEADO (alerta en Soporte con el bloque)
[Bloque N] aprobó → APROBADO: acceso, correo de contraseña, cierre del ticket de Obersuite
```

- `Landing(token)` devuelve: estado general, lista de bloques con su estado (para la barra de progreso) y el **bloque actual** completo (video + preguntas sin respuestas + intentos restantes + mínimo).
- `Submit(token, block_id, answers)` califica solo ese bloque. Si el `block_id` no es el bloque actual se rechaza: no se puede saltar ni volver a enviar uno aprobado.
- La barra de progreso vertical del panel izquierdo (que hoy muestra video/cuestionario/resultado) pasa a listar los bloques.

---

## 5. Panel de administración (Novedades → pestaña Inducción)

La pestaña se reorganiza en tres partes, todo superadmin:

1. **Interruptor global** y vigencia del enlace (lo que queda en `induction_configs`).
2. **Biblioteca de bloques**: lista con nombre, video, cantidad de preguntas y en qué programas se usa. Crear/editar bloque reutiliza el constructor de preguntas que ya existe en `InductionSettings.tsx`. Un bloque en uso no se puede borrar.
3. **Programas**: lista de programas; al abrir uno se ven sus bloques ordenables por arrastre (mismo patrón que el reorder de Novedades), el mínimo por defecto, los intentos por bloque, las empresas asignadas y el marcador de programa por defecto.

---

## 6. Soporte

- `Status(userID)` devuelve además el progreso por bloque y el programa. El panel del expediente lo pinta como lista con estado, intentos y mejor puntaje por bloque.
- El log de intentos indica el bloque.
- La alerta de bloqueo (`InductionAlertInput`) lleva `BlockName` y `BlockIndex` ("falló en el bloque 3 de 5: Seguridad de la información").

---

## 7. Fases y archivos

### Fase 1 — Backend (modelo, migración, servicio)

| Archivo | Trabajo |
|---|---|
| `backend/internal/models/induction.go` | Nuevos structs `InductionBlock`, `InductionProgram`, `InductionProgramBlock`, `InductionProgramCompany`, `InductionBlockProgress`; nuevos campos en `InductionInvite` y `InductionAttempt`; `InductionConfig` reducido; `Ready()` rediseñado. |
| `backend/internal/migrations/migrations.go` | Migración `202609251000_induction_blocks` con AutoMigrate + migración de datos de la sección 2. |
| `backend/internal/repository/induction_repository.go` | CRUD de bloques, programas, asignaciones y progreso; `ResolveProgramForCompany`; `GetDefaultProgram`. |
| `backend/internal/service/induction_service.go` | `InviteIfEnabled` resuelve programa y crea el snapshot + progreso; `Landing` devuelve bloque actual; `Submit` califica por bloque y avanza; `Reset` reinicia todo el progreso; `Status` incluye bloques; CRUD de bloques y programas con validaciones (programa activo con ≥1 bloque, un solo default, bloque en uso no se borra). |
| `backend/internal/service/ticket_service.go` | `InductionAlertInput` con bloque. |
| `backend/internal/handlers/induction.go` | Endpoints nuevos (sección 8). |
| `backend/internal/routes/platform_routes.go` | Rutas nuevas bajo `/inductions`. |
| `backend/internal/service/induction_service_test.go`, `induction_invite_test.go`, `onboarding_hire_test.go` | Adaptar y agregar casos: secuencia, intentos por bloque, bloqueo en bloque intermedio, resolución de programa por empresa, snapshot congelado. |

### Fase 2 — Landing pública

| Archivo | Trabajo |
|---|---|
| `frontend/src/services/induction.service.ts` | Nuevos tipos (`InductionLanding` con `blocks[]` y `current_block`), `submit` con `block_id`. |
| `frontend/src/pages/Induction.tsx` + `.module.css` | Wizard por bloque, barra de progreso de bloques, pantalla intermedia "aprobaste el bloque N, sigue el N+1". |

### Fase 3 — Panel de administración

| Archivo | Trabajo |
|---|---|
| `frontend/src/services/induction.service.ts` | CRUD de bloques, programas, asignaciones. |
| `frontend/src/components/Admin/InductionSettings.tsx` | Se parte en `InductionSettings.tsx` (interruptor + navegación), `InductionBlockLibrary.tsx`, `InductionBlockEditor.tsx` (constructor de preguntas actual), `InductionProgramList.tsx`, `InductionProgramEditor.tsx`. |
| `frontend/src/pages/Tutoriales.tsx` | Sin cambios de fondo; sigue montando `InductionSettings` en la pestaña. |

### Fase 4 — Soporte

| Archivo | Trabajo |
|---|---|
| `frontend/src/components/Admin/InductionStatusPanel.tsx` | Progreso por bloque, selector de programa al invitar. |
| `frontend/src/pages/AdminUserDetail.tsx` | Solo si cambian las props del panel. |

### Fase 5 — Documentación

| Archivo | Trabajo |
|---|---|
| `docs/integracion-obersuite.md` | Sección de inducción: qué cambia y qué no (el contrato de `/hire` no cambia). |
| Este documento | Pasar el estado a "implementado" y registrar desvíos. |

---

## 8. Endpoints

Panel (`/api/inductions`, acceso de Soporte; escritura solo superadmin):

| Método | Ruta | Uso |
|---|---|---|
| GET / PUT | `/config` | Interruptor global y vigencia (como hoy, con menos campos) |
| GET / POST | `/blocks` | Biblioteca de bloques |
| PUT / DELETE | `/blocks/:id` | Editar / borrar bloque (rechaza si está en uso) |
| GET / POST | `/programs` | Programas |
| GET / PUT / DELETE | `/programs/:id` | Detalle con bloques y empresas / editar / borrar |
| PUT | `/programs/:id/blocks` | Reemplaza la lista ordenada de bloques |
| PUT | `/programs/:id/companies` | Reemplaza las empresas asignadas |
| GET | `/users/:userId` | Estado con progreso por bloque |
| POST | `/users/:userId/invite` | Ahora acepta `program_id` opcional |
| POST | `/users/:userId/reset` | Igual que hoy |

Pública (`/api/induction`, con rate limit):

| Método | Ruta | Uso |
|---|---|---|
| GET | `/:token` | Landing con bloques y bloque actual |
| POST | `/:token/submit` | Body: `{ block_id, answers[] }` |

---

## 9. Riesgos y notas

- **Videos de inducción en Novedades**: al seguir saliendo de Novedades, un video creado para la inducción y publicado como visible se anuncia a toda su audiencia. La práctica es dejarlo **oculto** (`is_active = false`); la inducción lo lee igual porque va por id. Conviene indicarlo en el selector de video del bloque.
- **Borrar una novedad usada por un bloque**: el bloque queda sin video y pasa a ser solo cuestionario, que es un estado válido. No se bloquea el borrado.
- **Editar el cuestionario de un bloque con invitaciones en curso**: las preguntas se leen en vivo (como hoy), así que un cambio de preguntas afecta a quien está a mitad. El snapshot congela mínimo, intentos y orden, no las preguntas. Es el mismo comportamiento actual y se deja igual.
- **Un solo programa por defecto**: se garantiza con índice parcial único `WHERE is_default = true`.

---

## 10. Cómo quedó (desvíos respecto al diseño)

- **Snapshot y progreso en una sola tabla.** En vez de `blocks_json` en la
  invitación más `induction_block_progress`, existe `induction_invite_blocks`:
  una fila por invitación y bloque con el snapshot (nombre, video,
  cuestionario, mínimo) **y** el progreso (estado, intentos, mejor puntaje).
  Tienen el mismo grano y la misma vida, así que separarlos era duplicar.
- **Columnas retiradas.** La migración `202609251200_induction_blocks` quita
  `tutorial_id`, `survey_id`, `passing_score` y `max_attempts` de
  `induction_configs`, y `tutorial_id`, `survey_id`, `passing_score`,
  `attempts` y `best_score` de `induction_invites`. El rollback no las devuelve.
- **La invitación recuerda el nombre del programa** (`program_name`) además de
  su id, para que Soporte lo vea aunque el programa se borre después.
- **Reordenar bloques** en el programa va con flechas arriba/abajo, no con
  arrastre: la secuencia rara vez pasa de cinco o seis bloques.
- **El primer programa que se crea es el por defecto** aunque no se marque:
  sin uno, la inducción no tiene a quién mandar a nadie. El por defecto no se
  apaga, no se desmarca ni se borra; para cambiarlo se marca otro.
- **Un bloque nuevo nace con su cuestionario** (kind `induction`) creado desde
  el mismo editor, para que el constructor de preguntas aparezca sin pasos
  intermedios. Las preguntas se leen en vivo, como antes.
- **El expediente de Obersuite** (`GET .../:uid/onboarding`) suma `program_name`,
  `total_blocks`, `completed_blocks` y `blocks[]`; `best_score` y
  `passing_score` se conservan y son los del bloque en curso.
- **Soporte elige programa al invitar** (opcional): el selector del expediente
  precarga "automático", que resuelve por la empresa o el por defecto.

---

## 11. Insignias (25-sep-2026)

Reconocimientos **permanentes** que el profesional gana al aprobar pruebas de
la inducción. Viven en `user_badges` y no en la invitación, porque la
invitación se reemplaza al recontratar y se reinicia desde Soporte; lo ganado
no se pierde por eso. Índice único por usuario y origen (`source_key`):
reaprobar tras un reinicio no duplica.

| Origen | `kind` | `source_key` | Título | Aspecto |
|---|---|---|---|---|
| Aprobar un bloque | `block` | `block:<id>` | nombre del bloque | `badge_icon` / `badge_color` del bloque |
| Completar un programa | `program` | `program:<id>` | nombre del programa | `badge_icon` / `badge_color` del programa |
| Mérito "A la primera" | `merit` | `merit:first_try:<program>` | fijo | Zap / ámbar |
| Mérito "Impecable" | `merit` | `merit:perfect:<program>` | fijo | Crown / oro |

- **Méritos** (catálogo fijo en `service/badge_catalog.go`): "A la primera" =
  todos los bloques aprobados con un solo intento; "Impecable" = 100% en todos.
  Se evalúan al completar el programa.
- **Iconos y colores** son un set cerrado (16 iconos de lucide, 8 colores);
  el backend valida y cae al de respaldo. El aspecto se **copia** a la insignia
  al otorgarla: renombrar o borrar el bloque después no la cambia.
- **Se otorgan en `Submit`**, best-effort: una insignia que no se pudo guardar
  no frena la aprobación. Cada insignia nueva manda una notificación tipo
  `insignia` a la campanita.
- **Migración `202609251500_induction_badges`**: crea la tabla y otorga hacia
  atrás por bloques y programas ya aprobados, incluidos los méritos.

### Dónde se ven

| Superficie | Qué muestra |
|---|---|
| Landing (`/induccion/:token`) | Celebración con animación al aprobar (`badges_earned` del envío) y las ganadas en el panel de marca (`badges` de la landing) |
| Perfil del profesional | Tarjeta "Insignias" con ganadas y **por ganar** en gris (`GET /me/badges`) |
| Expediente en Admin | Misma tarjeta, con pendientes (`GET /users/:id/badges`) |
| Ficha del empleado (empresa) | Solo ganadas; se oculta si no hay ninguna (misma ruta, visibilidad del detalle de usuario) |
| Editores de bloque y programa | Sección "Insignia" con selector de icono, color y vista previa |

Componentes: `frontend/src/components/Badges/` (`badgeCatalog.ts`,
`BadgeMedallion`, `BadgePicker`, `UserBadges`).

---

## 12. Ingreso vs. capacitación: el bloqueo del acceso (25-sep-2026)

El bloqueo del acceso hasta aprobar nació para el ingreso desde Obersuite. Con
programas reutilizables e insignias, mandarle una capacitación a alguien que
ya trabaja no debe sacarlo de la plataforma. La decisión vive en la
**invitación** (`induction_invites.gates_access`), no en el programa: el mismo
programa puede ser ingreso para uno y repaso para otro.

| Vía | `gates_access` | Efecto |
|---|---|---|
| `/hire` desde Obersuite | siempre `true` | Sin acceso hasta aprobar (como siempre) |
| Envío desde Soporte | lo que marque la casilla "Bloquear el acceso hasta que apruebe" (por defecto **no**) | Sigue entrando; al aprobar gana insignias |
| Envío desde Soporte a quien nunca tuvo acceso (`pending` / `blocked`) | forzado a `true` | No hay capacitación "abierta" para quien no puede entrar |

Consecuencias del modo sin bloqueo: aprobar **no** toca `onboarding_status`,
no manda el correo de contraseña ni cierra el ticket de incorporación; agotar
intentos deja la invitación bloqueada y abre la alerta en Soporte como
"Capacitación no aprobada" (acceso sin cambios); el reinicio conserva el modo.
Quien ya aprobó su ingreso puede recibir capacitaciones, pero no una
invitación que lo bloquee (para eso está el reinicio).

Como la persona está dentro, al invitarla sin bloqueo recibe una notificación
tipo `capacitacion` con el enlace, y su perfil muestra "Capacitación
pendiente: N de M bloques" con botón para continuar (`GET /me/induction`).
La landing adapta los textos según `gates_access`.

Migración: `202609251700_induction_gates_access` (todo lo anterior queda como
ingreso por el default de la columna).

### Varias capacitaciones por persona (25-sep-2026)

Una persona acumula invitaciones con el tiempo (el ingreso y las
capacitaciones que le manden), con **una sola en curso a la vez**:

- El índice único por usuario pasó a ser **parcial** (solo `pending`):
  migración `202609251800_induction_multiple_invites`.
- Enviar una **capacitación** con otra pendiente se rechaza; un **ingreso**
  reemplaza la pendiente (re-contratación). Las terminadas (aprobadas o
  bloqueadas) se conservan como historial y nunca se borran.
- `GetInviteByUser` devuelve la **actual**: la pendiente si la hay, si no la
  más reciente. `Status` trae además `history[]` con todas y `invite_id`.
- Reiniciar: `POST /inductions/users/:id/reset` reinicia la actual;
  `POST /inductions/invites/:id/reset` una concreta. Una aprobada no se
  reinicia (para repetirla se envía de nuevo), y no se puede reabrir una
  bloqueada si hay otra pendiente.
- El panel de Soporte muestra la actual en detalle, "Todas las capacitaciones"
  como historial y, cuando no hay ninguna en curso, "Enviar otra capacitación".

