# Plan de implementación: motor de Workflows en Obertrack

**Versión 2 · 20 de agosto de 2026 · Osvell Chacón**

Ajusta y reordena el informe técnico de viabilidad
([analisis-workflows.md](analisis-workflows.md)) al alcance de producto definido.
Incorpora las decisiones A1–A6 de la adenda del 20 de agosto, posteriores a la revisión
del plan contra el código.

---

## 1. Resumen ejecutivo

Obertrack incorporará un motor de automatizaciones sobre el módulo de **Tareas**, que
permitirá a empresas, managers y supervisores definir reglas del tipo
`disparador → condiciones → acciones` sin intervención de desarrollo.

| | |
|---|---|
| Alcance | Módulo de Tareas exclusivamente |
| Usuarios que configuran | Empleador, manager, supervisor |
| Usuarios que reciben | Todos, incluidos profesionales |
| Primer entregable en producción | **Día 11** |
| Acciones que modifican tareas | Día 14 |
| Constructor de reglas para el usuario | Día 23 |
| Total del alcance completo | **26 días** de un desarrollador |

El cambio de criterio respecto al informe técnico es el orden: **primero recetas
preconfiguradas, después constructor visual**. El informe planteaba construir el
constructor antes de tener una sola automatización en producción, lo que retrasaba todo
valor visible al día 14 y comprometía 6 días de trabajo a un diseño de interfaz sin
evidencia de uso.

---

## 2. Alcance de la versión 1

### Dentro

- Disparadores sobre tareas: creación, cambio de estado, cambio de prioridad,
  asignación, comentario nuevo, fecha de vencimiento próxima, antigüedad en columna.
- Acciones de aviso: campanita, mensaje directo del bot en el chat, correo.
- Acciones sobre la tarea: cambiar prioridad, cambiar estado, asignar, comentar,
  crear tarea.
- Reglas por empresa (tenant). Historial de ejecuciones consultable.

### Fuera de la versión 1

| Excluido | Motivo |
|---|---|
| Disparadores sobre Horas (`workhour.*`) | Fuera del módulo de Tareas |
| Recordatorio de horas no registradas | Ya existe en `inactivity_watcher.go` |
| Reglas globales de plataforma (superadmin) | Cambiaría el modelo de datos; sin demanda |
| Disparador `task.restored` | `trashService.Restore` no pasa por `taskService` |
| Acciones hacia servicios externos (Slack, WhatsApp, Zoho) | Fase posterior |
| Ramificaciones condicionales (if/else entre pasos) | Pasos en secuencia lineal en v1 |

---

## 3. Catálogo funcional

### 3.1 Disparadores

| Clave | Descripción | Fase |
|---|---|---|
| `task.created` | Se crea una tarea en un tablero | 1 |
| `task.status_changed` | La tarea cambia de columna | 1 |
| `task.priority_changed` | Cambia la prioridad | 1 |
| `task.assigned` | Se agrega o cambia un asignado | 1 |
| `task.comment_added` | Alguien comenta la tarea | 2 |
| `schedule.task_due` | Faltan N horas para la fecha fin | 3 |
| `schedule.task_stale` | La tarea lleva N días en la misma columna | 3 |
| `board.member_joined` | Alguien se une al tablero | 5 |

### 3.2 Condiciones

Árbol de `todas` / `alguna` sobre: tablero, columna de origen, columna de destino,
prioridad, si tiene o no responsable, si tiene o no fecha fin, si está vencida, rol del
actor del cambio, si el actor es el sistema.

Las condiciones sobre columnas se atan al par **`(board_id, phase_id)`**, nunca a la
cadena de `status` suelta (ver §6, C2).

### 3.3 Acciones

| Clave | Efecto | Fase |
|---|---|---|
| `notify` | Notificación en campanita + WebSocket + Web Push | 1 |
| `chat_dm` | Mensaje directo del bot Obertrack | 1 |
| `email` | Correo vía Brevo, con interruptor por tipo | 1 |
| `task_set_priority` | Cambia la prioridad de la tarea | 2 |
| `task_assign` | Asigna un responsable | 2 |
| `task_comment` | Publica un comentario del sistema | 2 |
| `task_set_status` | Mueve la tarea de columna | 2 |
| `task_create` | Crea una tarea nueva | 5 |

### 3.4 Destinatarios — se resuelven en tiempo de ejecución

**Regla de diseño obligatoria:** una acción de aviso nunca almacena un `user_id` como
destinatario principal. Los destinatarios se resuelven al ejecutar, a partir del rol.
Esto hace que las reglas sobrevivan a renuncias, cambios de manager y reasignaciones de
tablero.

| Clave | Se resuelve como |
|---|---|
| `asignados` | `task_users` de la tarea |
| `manager_del_asignado` | `resolveManagersFor(asignado, tenant)` — ver §6, C4 |
| `supervisor_del_tablero` | Miembros del tablero con `is_supervisor` |
| `creador_de_la_tarea` | `tasks.created_by` |
| `creador_del_tablero` | `boards.created_by` |
| `empleador` | Cuenta `user_type='empleador'` del tenant |
| `actor` | Quien provocó el cambio |
| `usuario_fijo` | Un `user_id` concreto — disponible, pero no es la opción por defecto |

**Cadena de respaldo del "líder del proyecto"**: `manager_del_asignado` →
`supervisor_del_tablero` → `creador_del_tablero` → `empleador`. Si ningún nivel resuelve
a un usuario activo, el paso se marca `skipped` con motivo, nunca falla en silencio.

### 3.5 Variables interpoladas

```
{{tarea.titulo}}  {{tarea.estado}}  {{tarea.estado_anterior}}  {{tarea.prioridad}}
{{tarea.fecha_fin}}  {{tarea.enlace}}  {{tarea.asignados}}  {{tarea.dias_en_columna}}
{{tablero.nombre}}  {{actor.nombre}}  {{pasos.N.*}}
```

Se resuelven contra el snapshot guardado en `workflow_runs.context`, no releyendo la base
de datos, para que un reintento produzca exactamente el mismo resultado.

---

## 4. Permisos

### 4.1 Quién configura

| Perfil | Alcance de configuración |
|---|---|
| Empleador | Todos los tableros del tenant |
| Manager | Solo tableros donde es miembro |
| Supervisor | Solo tableros donde es miembro |
| Profesional | Sin acceso de configuración |
| Superadmin | Acceso técnico de diagnóstico, no crea reglas de negocio |

Este alcance es **más estricto** que el del módulo de Tareas y no existe hoy en el
código: se construye desde cero (ver §6, C5).

### 4.2 Quién consulta

El profesional ve, en la ficha de la tarea, qué automatizaciones la afectaron y cuándo.
No ve el catálogo completo de reglas del tenant.

### 4.3 Dos reglas de seguridad innegociables

1. **El módulo `workflows` nace fail-closed.** El RBAC actual no restringe a un usuario
   sin roles asignados (`handlers/rbac.go:61-66`). Ese comportamiento por defecto es
   aceptable para consultar tareas y es inaceptable para un módulo que puede modificar
   tareas ajenas y disparar correos masivos. `workflows` requiere permiso explícito.
2. **El runner valida alcance, no solo el endpoint.** Una regla solo puede ejecutar
   acciones sobre tableros dentro del alcance de quien la creó. Sin esta validación *en
   el momento de ejecutar*, un manager escala privilegios creando una regla que toca
   tableros donde no es miembro. Validar únicamente al guardar la regla no basta: el
   creador puede perder acceso al tablero después.

---

## 5. Decisiones

### 5.1 Tomadas

| # | Decisión |
|---|---|
| D1 | Alcance limitado al módulo de Tareas |
| D2 | Configuran empleador, manager y supervisor; el profesional solo recibe |
| D3 | Destinatarios por rol, resueltos en ejecución (§3.4) |
| D4 | "Líder del proyecto" = manager del asignado, con cadena de respaldo |
| D5 | Reglas por tenant; sin reglas globales de plataforma en v1 |
| D6 | Recetas preconfiguradas antes que constructor visual |
| D7 | Toda regla nace apagada y requiere activación explícita |
| **D8** | La idempotencia se apoya en una columna `tasks.revision`, no en un historial genérico de cambios (A2) |
| **D9** | Los avisos duplicados se resuelven deduplicando en el destino, no suprimiendo el aviso nativo en origen (A3) |
| **D10** | La resolución de managers pasa por un helper único que respeta el flag `MultiManagerReads` (A4) |
| **D11** | La incoherencia de alcance con el módulo de Tareas se documenta y no se alinea en este proyecto (A5) |
| **D12** | La Fase 1 mantiene los cuatro disparadores y sube a 8 días; la pantalla de activación se recorta (A6) |

### 5.2 Pendientes de aprobación

**P2 — Renombrar una columna.** Las reglas se atan a `(board_id, phase_id)`, pero el
`status` se deriva del nombre de la fase (`service/phase_status.go:26-38`). Renombrar una
columna puede dejar reglas inconsistentes.

- *Opción A*: bloquear el renombrado si hay reglas activas que la referencian.
- *Opción B (recomendada)*: permitir el renombrado y marcar las reglas afectadas con una
  advertencia visible en la lista, sin apagarlas.

**P3 — Límites operativos.** Máximo de reglas por tenant, máximo de pasos por regla y
máximo de ejecuciones por minuto. El worker comparte proceso con la cola de Google
Calendar; una regla mal escrita sobre un tablero grande puede retrasarla. Propuesta: 50
reglas por tenant, 10 pasos por regla, 300 ejecuciones por minuto por tenant.

Ninguna de las dos bloquea las Fases 0 ni 1.

> **P1 (convivencia con los avisos actuales) queda resuelta** por A3 → ver §6, C3.

---

## 6. Correcciones al informe técnico

Estado tras la revisión del plan contra el código y la adenda A1–A6.

### C1 — Idempotencia: resuelta con `tasks.revision` ✅

El informe definía `dedup_key = sha1(disparador + entidad + versión)` sin que existiera
una fuente de versión. La revisión del código encontró además que el hueco alcanzaba a
dos disparadores de la Fase 1: `task.priority_changed` no deja rastro en ninguna tabla, y
`task_users` tiene clave primaria compuesta `(task_id, user_id)`, sin `id` ni timestamps,
con `SyncAssignees` haciendo `Association.Replace` (borra todo e inserta todo).

**Decisión (A2):** columna `tasks.revision` (`int not null default 0`), incrementada en el
mismo `UPDATE` que cualquier mutación de la tarea con `gorm.Expr("revision + 1")`, nunca
como escritura separada.

```
dedup_key = sha1(trigger_type + "task" + task_id + revision)
```

Como el tipo de disparador forma parte de la clave, un `UPDATE` que cambie estado y
prioridad a la vez genera dos claves distintas con la misma revisión — que es el
comportamiento correcto. El emisor lee `revision` del `finalTask` ya recargado, no del
objeto previo.

**No** se generaliza `task_status_history` a un `task_change_history`: eso obligaría a
diferenciar conjuntos de asignados antes y después dentro de `SyncAssignees`, y a decidir
el formato de `from`/`to` para campos que no son escalares. Sube la Fase 0 por encima de
los 3 días y agrega una tabla que crece rápido, para resolver un problema que una columna
entera resuelve.

`task_status_history` se mantiene **acotada al estado**: su razón de ser es el valor de
producto (antigüedad en columna, "En proceso desde hace 4 días") y el disparador
`schedule.task_stale`, no la idempotencia.

*Nota para la Fase 5*: `board.member_joined` no tiene revisión de tarea. Su clave se
deriva de `(board_id, user_id, fecha del vínculo)`.

### C2 — Fases compartidas entre tableros: descartado ✅

El informe advertía que `Phase` no tiene `TenantID` ni `DeletedAt` y se vincula por la
tabla puente `board_phases`, con el riesgo de que una regla atada a `phase_id` se
disparara con tareas de otro tablero o de otro tenant.

**Verificado en el código, en todos los caminos de escritura:**

- `board_service.go:297-303` — crear un tablero instancia una `Phase` **nueva** por columna
- `board_service.go:377-383` — `AddPhase` también crea fila nueva
- `board_repository.go:103-114` — `AddPhase` hace `Create(phase)` y sólo después la vincula
- `cmd/seed/data.go:504` — el seeder, igual

No existe ningún camino que vincule una fila de `phases` existente a un segundo tablero:
el many2many modela en realidad un 1-N.

**Decisión (A1):** se elimina el medio día de verificación previa a la Fase 1. Se
**mantiene** el par `(board_id, phase_id)` en las condiciones y el filtro por `tenant_id`
en todas las consultas del runner: no cuesta nada y protege el día en que alguien reutilice
una fase. R1 baja de rojo a verde.

*Deuda técnica registrada aparte*: `board_repository.go:116-118` (`RemovePhase`) borra sólo
el vínculo, no la fila, así que `phases` acumula huérfanas. Como no cuelgan de ningún
tablero, no hay fuga posible. **Ticket independiente, fuera de este proyecto.**

### C3 — Avisos duplicados: resuelta con deduplicación en el destino ✅

La opción C original ("suprimir el aviso nativo cuando exista una regla activa que cubra
ese mismo evento y destinatario") exigía saber dentro del request si las condiciones de la
regla evalúan a verdadero. Eso contradice el diseño del emisor, que sólo encola y deja la
evaluación al worker para no meter latencia en `PUT /tasks/:id`.

**Decisión (A3):** deduplicar en `NotificationService.CreateNotification` por
`(user_id, type, data.task_id)` dentro de una ventana corta — **60 segundos** como valor
inicial, configurable. El primero en llegar gana. Una sola función tocada y sin cambio de
comportamiento para quien no use workflows.

**A verificar antes de la Fase 2:** si la ruta nativa de aviso de tarea envía también
correo o DM de chat, la ventana debe aplicarse al aviso como **unidad lógica** y no sólo al
canal de campanita. Si el aviso nativo es únicamente notificación interna, C″ basta tal cual.

### C4 — `manager_del_asignado` no tiene hoy una fuente única ⚠️

D4 lo definía como `employments.manager_id`, pero el proyecto tiene **tres** fuentes y cuál
se lee depende de un feature flag:

- `users.manager_id`
- `employments.manager_id`
- `employment_managers` (N-a-N), cuando `MultiManagerReads` está encendido
  (`service/flags.go:13-20`)

El flag está **encendido en desarrollo y apagado en producción**
(`docker-compose.override.yaml`). Sin un resolutor único que lo respete, la misma regla
avisa a personas distintas en dev y en prod. No existe tal helper: `countManagerReports`
(`service/tenant_helpers.go:24-45`) sólo cuenta, y combina fuentes tomando el máximo.

**Decisión (A4):** `resolveManagersFor(userID, tenantID) []uint` en `tenant_helpers.go`,
con la misma disciplina de flag que el resto del código. **Medio día, dentro de la Fase 1.**

### C5 — El alcance del manager es funcionalidad nueva ⚠️

§4.1 asume que un manager sólo alcanza los tableros donde es miembro. El sistema dice hoy
lo contrario en dos sitios:

- `boardService.GetAll:140` — cualquier manager ve **todos** los tableros del tenant
- `canModifyTask:151-153` — `if isEmployerRole(role) || isManager { return true }`:
  cualquier manager puede modificar cualquier tarea del tenant

**Decisión (A5):** se confirma la restricción más estricta para workflows y se cuenta como
funcionalidad nueva, no como reutilización. La incoherencia con el módulo de Tareas **se
documenta y no se alinea ahora**: alinearla es un cambio de comportamiento en una función
en producción, ajeno a este proyecto. **Ticket independiente.**

### C6 — El fail-open del RBAC ⚠️

Detallado en §4.3. El informe lo reporta como dato neutro; para este módulo es un riesgo de
seguridad y se trata como tal.

---

## 7. Plan por fases

Estimaciones para **un desarrollador**, con pruebas incluidas. La suite de Go corre en el
contenedor `golang:1.25-alpine`.

### Fase 0 — Historial de estado + `revision` · 3 días

Prerrequisito de todo lo demás.

- Tabla `task_status_history` (acotada al estado: `from_status`, `to_status`,
  `changed_by`, `changed_at`).
- Columna `tasks.status_changed_at`, sellada en el mismo statement que el cambio de estado.
- Columna `tasks.revision`, incrementada con `gorm.Expr("revision + 1")` dentro del mismo
  `Updates(...)` de cualquier mutación (C1).
- Migración de relleno: `status_changed_at = updated_at` para lo existente. No se inventan
  filas de historial — la bitácora arranca vacía y se llena con los movimientos futuros.
- Escritura desde `taskService.Create`, `Update` y `ToggleCompletion`.

*Archivos*: `models/task.go`, `models/task_status_history.go`,
`repository/task_repository.go`, `service/task_service.go`, `migrations/migrations.go`.

**Entregable independiente:** la ficha de la tarea muestra "En proceso desde hace 4 días".
Aporta valor aunque el motor nunca se construyera.

### Fase 1 — Motor + recetas en producción · 8 días · depende de Fase 0

El corazón del sistema, sin constructor visual.

- Modelos, repositorio y runner, copiando el patrón de `CalendarSyncService` (tick + canal
  `nudge`, backoff 30 s → 2 h, 6 intentos), **con `FOR UPDATE SKIP LOCKED` desde el primer
  commit**.
- Emisor de eventos cableado en `deps.go` con el patrón `SetCalendarSync`.
- Disparadores `task.created`, `task.status_changed`, `task.priority_changed`,
  `task.assigned`.
- Acciones `notify`, `chat_dm`, `email`.
- Resolución de destinatarios por rol con cadena de respaldo (§3.4), incluido
  `resolveManagersFor` (C4).
- Antibucle (`depth` máximo 3, `cause_run_id`) e idempotencia (C1) desde el inicio:
  retrofitearlos después es caro.
- Deduplicación de avisos en `CreateNotification` (C3).
- Tablas del motor añadidas a `auditSkipTables` para no inflar `audit_logs`.
- **4 recetas precargadas** por tenant, apagadas por defecto:
  1. Tarea en "En proceso" sin responsable → avisar al manager del asignado.
  2. Tarea marcada urgente → avisar al asignado y a su manager.
  3. Tarea asignada a alguien → avisar al nuevo responsable por chat.
  4. Tarea creada sin fecha fin → avisar al creador del tablero.
- Pantalla de activación **mínima**: lista de recetas, interruptor y selección de tablero.
  **Sin selector de destinatario** — cada receta se publica con el destinatario por defecto
  de su cadena de respaldo. La elección de destinatario llega con el constructor (Fase 4).

No se recorta a dos disparadores: el costo está en el armazón, no en cada disparador, y
agregar los otros dos después obliga a repetir el ciclo de pruebas del emisor completo. Es
más barato pagar dos días ahora que hacer la Fase 1 dos veces.

**⭐ Entregable en producción: día 11.** Las empresas ya tienen automatizaciones
funcionando y se empieza a medir cuáles activan.

### Fase 2 — Acciones que modifican tareas · 3 días · depende de Fase 1

`task_set_priority`, `task_assign`, `task_comment`, `task_set_status`. Requiere actor de
sistema (`users.is_system` ya existe) o un método `UpdateAsSystem`, la verificación de datos
obsoletos antes de mutar, y escritura por ID con modelo limpio para no repetir el problema
conocido de GORM en `taskRepository.Update`. Se añade el disparador `task.comment_added`.

Antes de empezar, cerrar la verificación pendiente de C3 (si el aviso nativo usa más de un
canal).

**Entregable: día 14.** "Sube la prioridad automáticamente" es de lo más pedido y en el plan
original llegaba al día 22.

### Fase 3 — Disparadores por tiempo · 3 días · depende de Fase 2

`workflow_schedule_watcher` con hora y zona horaria configurables y deduplicación por
período, **extrayendo** el patrón de `ReportMailWatcher` en lugar de copiarlo — sería el
octavo watcher con el mismo bucle escrito a mano. Habilita `schedule.task_due` y
`schedule.task_stale`, este último posible únicamente gracias a la Fase 0.

**Entregable: día 17.** Recordatorios de vencimiento y detección de tareas estancadas.

### Fase 4 — API y constructor de reglas · 6 días · depende de Fase 3

Ahora sí, con datos de uso reales de las Fases 1 a 3.

Endpoints bajo `/api/workflows` (lista, CRUD, encender/apagar, `/test` en seco, historial de
ejecuciones, reintento manual y `/catalog`). El endpoint `/catalog` es obligatorio: evita que
el frontend replique la lógica de disparadores y acciones, que es el mismo error que ya
obliga a mantener `phase_status.go` y `phaseStatus.ts` sincronizados a mano.

Módulo RBAC `workflows` fail-closed (§4.3, C6) y validación de alcance en el runner, contada
como funcionalidad nueva (C5). Frontend: constructor de disparador → condiciones → pasos, con
previsualización de variables, selector de destinatario y botón de prueba.

**Entregable: día 23.** El usuario crea sus propias reglas sin desarrollo.

### Fase 5 — Observabilidad y cierre · 3 días · depende de Fase 4

Vista de ejecuciones con filtro por estado y motivo de los `skipped`, reintento manual,
apagado automático de una regla tras N fallos consecutivos, purga de ejecuciones antiguas,
límites de P3, disparador `board.member_joined` (enganchado en el repositorio por sus cinco
puntos de llamada) y acción `task_create`.

**Entregable: día 26.** El motor se diagnostica sin entrar a la base de datos.

### Cronograma

| Fase | Días | Acumulado | Hito |
|---|---|---|---|
| 0 · Historial de estado + `revision` | 3 | 3 | Antigüedad visible en la tarea |
| **1 · Motor + recetas** | **8** | **11** | **Primeras automatizaciones en producción** |
| 2 · Acciones sobre tareas | 3 | 14 | El motor modifica, no solo avisa |
| 3 · Disparadores por tiempo | 3 | 17 | Vencimientos y tareas estancadas |
| 4 · API + constructor | 6 | 23 | El usuario crea sus reglas |
| 5 · Observabilidad | 3 | 26 | Operable y con límites |

---

## 8. Criterios de aceptación

Una fase se considera terminada cuando:

1. Existen pruebas de aislamiento por tenant sobre todo lo que agregue.
2. Ninguna regla puede disparar una cadena de más de 3 niveles, con prueba que lo demuestre.
3. Un mismo evento encolado dos veces produce una única ejecución.
4. Un reintento sobre una ejecución parcialmente completada no repite los pasos ya
   ejecutados.
5. Una regla del tenant A no lee ni modifica entidades del tenant B.
6. El apagado de una regla surte efecto sobre las ejecuciones aún pendientes en cola.

---

## 9. Riesgos vigentes

| # | Riesgo | Estado | Mitigación |
|---|---|---|---|
| R1 | Fases compartidas entre tableros permitirían fuga entre tenants | 🟢 Descartado | C2/A1: verificado que no se comparten. Se mantiene la defensa por diseño: reglas atadas a `(board_id, phase_id)` y filtro por `tenant_id` |
| R2 | Módulo nuevo hereda el fail-open del RBAC | 🔴 Vigente | §4.3 / C6: `workflows` fail-closed, validación de alcance en el runner |
| R3 | Avisos duplicados con las notificaciones existentes | 🟢 Resuelto | C3/A3: deduplicación en `CreateNotification`, ventana de 60 s |
| R4 | Una sola instancia de backend asumida en el código | 🟠 Vigente | `FOR UPDATE SKIP LOCKED` desde la Fase 1; si se escala, hace falta elección de líder para los watchers |
| R5 | El worker comparte proceso con la cola de Google Calendar | 🟠 Vigente | Límites de P3; goroutine y tick propios |
| R6 | App móvil no evaluable (código Dart ausente del checkout) | 🟡 Vigente | Verificar si consume `PUT /api/tasks/:id`; si sí, queda cubierta sin cambios |
| R7 | Renombrar una columna deja reglas inconsistentes | 🟡 Vigente | P2, opción B |
| R8 | `manager_del_asignado` resuelve distinto en dev y en prod | 🟠 Vigente | C4/A4: `resolveManagersFor` respetando `MultiManagerReads`, en la Fase 1 |
| R9 | El alcance del manager en workflows no existe hoy y diverge del módulo de Tareas | 🟡 Aceptado | C5/A5: se construye nuevo y la divergencia se documenta; alinear Tareas es ticket aparte |

---

## 10. Orden de trabajo

1. ~~Incorporar A1 a A6 al plan y guardarlo como `docs/plan-workflows.md`.~~ ✅ Este documento.
2. **Arrancar la Fase 0**: `task_status_history`, `tasks.status_changed_at`,
   `tasks.revision`, migración de relleno y escritura desde `taskService.Create`, `Update`
   y `ToggleCompletion`.
3. P2 (renombrado de columnas) y P3 (límites operativos) siguen pendientes de aprobación,
   pero no bloquean las Fases 0 ni 1.

### Tickets independientes derivados de este plan

- Filas huérfanas en `phases` tras `RemovePhase` (C2).
- Divergencia de alcance del manager entre workflows y el módulo de Tareas (C5).

---

# 11. Adenda · Modo Workflow (puertas de fase)

**21 de agosto de 2026.** Amplía el alcance con un segundo tipo de automatización,
pedido después de ver la Fase 1 funcionando.

## 11.1 Qué es, y en qué se diferencia de lo construido

El **Modo Libre** se mantiene intacto: sin configuración, el tablero se arrastra como
siempre. El **Modo Workflow** es una capa opcional por `(tablero, fase)` que convierte
una columna en un punto de control.

La diferencia con el motor de la Fase 1 no es de tamaño, es de naturaleza:

| | Fase 1 (reactivo) | Modo Workflow (puerta) |
|---|---|---|
| Cuándo actúa | **después** del cambio | **antes**, lo intercepta |
| Bloquea | nunca | sí: sin datos, la tarjeta no avanza |
| Dónde corre | worker, en segundo plano | dentro del request |
| Si falla | reintenta con backoff | el movimiento se rechaza |

El flujo es: **disparador** (intento de mover a una fase con puerta) → **interrupción**
(formulario) → **validación** (campos obligatorios) → **consecuencia** (acción según lo
respondido) → **registro** en el historial de la tarea.

## 11.2 Lo que se reaprovecha

Bastante más de lo que sugiere la diferencia conceptual:

- `Workflow` ya está acotada a `(board_id, phase_id)`, con condiciones, pasos,
  interruptor y bitácora. La puerta es un `trigger_type` nuevo, no una entidad nueva.
- El catálogo de acciones, la resolución de destinatarios y el antibucle sirven tal
  cual: pasan a ser la *consecuencia* de la puerta.
- `task_status_history` (Fase 0) es exactamente donde encaja "quién aprobó, cuándo y
  qué adjuntó". Sólo hay que añadirle el formulario enviado.
- La pantalla de Automatizaciones y su portero fail-closed no cambian.

Lo único que **no** sirve es el worker asíncrono como camino de decisión: una puerta no
puede resolverse quince segundos más tarde.

## 11.3 La decisión que separa una puerta de un adorno

El modal es interfaz. Si la regla vive sólo en el frontend, cualquiera la salta con un
`PUT /api/tasks/:id` — y la app móvil ya consume ese endpoint. **La puerta se aplica en
el servidor**, y el modal es sólo su presentación.

```
PUT /tasks/:id {status: "en_revision"}
   ← 422 + definición del formulario     (la fase tiene puerta y no vino relleno)
PUT /tasks/:id {status: "en_revision", gate: {...respuestas}}
   → valida → aplica → registra → dispara consecuencias
```

Que el cliente **descubra** el formulario desde la respuesta tiene un efecto secundario
valioso: la app móvil respeta puertas nuevas sin que nadie la actualice.

## 11.4 Decisiones tomadas

| # | Decisión |
|---|---|
| G1 | Se reutiliza `Workflow` con `trigger_type = "task.entering_phase"` y una columna `form_schema`, en vez de una entidad paralela: una sola pantalla, un solo registro de ejecuciones |
| G2 | Enforcement **en el servidor**, con la definición del formulario viajando en el 422 |
| G3 | Tipos de campo en v1: **texto, enlace, selección, adjunto, fecha y número** |
| G4 | La puerta aplica **a todos, sin excepciones de rol** — empleador y managers incluidos |
| G5 | El formulario enviado se guarda en `task_status_history`, junto al movimiento que lo motivó |
| G6 | Los adjuntos se suben ANTES por `/api/uploads` (que ya existe) y el formulario envía la URL: así el movimiento no tiene que ser transaccional con un fichero |

**G5 tiene una consecuencia que no se puede pasar por alto:** hoy el historial se escribe
best-effort, con el error registrado pero sin tumbar la operación. Un formulario de puerta
es un registro con peso —quién aprobó qué— así que en este camino la escritura pasa a ser
**transaccional** con el cambio de estado: si no se puede registrar, el movimiento no ocurre.

## 11.5 Riesgos propios

| # | Riesgo | Mitigación |
|---|---|---|
| G-R1 | Una puerta mal configurada deja una columna inalcanzable y bloquea el trabajo | La puerta nace apagada; validación del esquema al guardar; el mensaje del 422 dice siempre qué falta |
| G-R2 | El tablero mueve la tarjeta de forma optimista y reordena después: un rechazo debe revertir el movimiento y NO lanzar el reordenamiento | Se resuelve en `useTasks.moveTask`, con prueba del camino rechazado |
| G-R3 | Sin excepciones de rol (G4), el empleador también queda atrapado si la puerta pide algo que no puede aportar | Es la decisión tomada; se vigila con el registro de rechazos y se revisa si molesta |
| G-R4 | Adjuntos huérfanos cuando la puerta acaba rechazando | Aceptado: es el comportamiento que ya tienen las subidas actuales |

## 11.6 Plan

La consecuencia automática que describe el concepto —mover a finalizado, devolver a por
hacer, etiquetar al responsable— **son las acciones de la Fase 2**, que aún no existen.
Pero la puerta entrega valor sin ellas: "no puedes mover esto sin rellenar aquello, y
queda registrado" ya es el punto de control. De ahí el orden.

| Fase | Alcance | Días |
|---|---|---|
| **G1 · Puerta que pide y registra** | `form_schema`, validación en servidor, 422 con la definición, modal en el tablero, escritura transaccional en el historial y su vista en la ficha | 5 |
| **G2 · Acciones que mutan** | La Fase 2 del plan original: `task_set_priority`, `task_assign`, `task_comment`, `task_set_status` | 3 |
| **G3 · Consecuencia según la respuesta** | Enrutado por el valor de un campo (si *aprueba* → finalizado; si *rechaza* → por hacer + aviso). No es un if/else general: es un campo de selección cuyas opciones apuntan a un destino | 3 |
| **G4 · Constructor de puertas** | Definir el formulario desde la interfaz, sin tocar código | 5 |

**Entregable mínimo con valor: G1, 5 días.** A partir de ahí cada fase se sostiene sola.
