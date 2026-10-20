// Package realtime avisa a los clientes conectados (web y app móvil) de que un
// dato cambió, para que recarguen lo que tienen en pantalla sin esperar a que
// alguien refresque a mano.
//
// No hace falta tocar cada servicio: un callback de GORM ve toda escritura en
// las tablas vigiladas (tareas, tableros, jornadas…) venga de donde venga
// (handler, automatización, worker). El aviso no lleva contenido, solo QUÉ
// cambió y de qué empresa: cada cliente vuelve a pedirlo por REST con sus
// propios permisos, así nadie ve algo que no le corresponde.
package realtime

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"
)

// EventType es el tipo del mensaje en el socket de notificaciones.
const EventType = "data_changed"

// Entidades que entienden los clientes.
const (
	EntityTask     = "task"
	EntityBoard    = "board"
	EntityWorkHour = "work_hour"
	EntityTeam     = "team"
)

// Notifier es lo que el emisor necesita del hub de notificaciones.
type Notifier interface {
	OnlineUserIDs() []uint
	NotifyUser(userID uint, notifType string, data interface{})
}

// watch dice, por tabla, qué entidad avisa y cómo se llega a su empresa
// cuando la fila no trae tenant_id: por la tabla y el campo del padre.
type watch struct {
	entity      string
	parentTable string // tabla con tenant_id a la que apunta parentField
	parentField string // campo de la fila con el id del padre
}

var watched = map[string]watch{
	"tasks":             {entity: EntityTask},
	"task_users":        {entity: EntityTask, parentTable: "tasks", parentField: "TaskID"},
	"comments":          {entity: EntityTask, parentTable: "tasks", parentField: "TaskID"},
	"task_attachments":  {entity: EntityTask, parentTable: "tasks", parentField: "TaskID"},
	"boards":            {entity: EntityBoard},
	"board_members":     {entity: EntityBoard, parentTable: "boards", parentField: "BoardID"},
	"board_phases":      {entity: EntityBoard, parentTable: "boards", parentField: "BoardID"},
	"board_invitations": {entity: EntityBoard, parentTable: "boards", parentField: "BoardID"},
	"phases":            {entity: EntityBoard},
	"work_hours":        {entity: EntityWorkHour},
	"employments":       {entity: EntityTeam},
}

// change es lo que se sabe de un cambio al verlo pasar.
type change struct {
	entity   string
	tenantID uint
	// De quién es la jornada que cambió (solo work_hours). 0: no se sabe, p.
	// ej. una aprobación en lote hecha con Where("id IN ?").
	subject uint
	// Cuando la fila no trae empresa, cómo buscarla al despachar.
	lookupTable string
	lookupID    uint
}

// Emitter agrupa los cambios y los despacha con un pequeño retraso: así el
// aviso sale después del COMMIT (el cliente recarga y ya ve el dato nuevo) y
// una ráfaga (mover diez tarjetas, aprobar un lote de horas) es un solo aviso.
type Emitter struct {
	db    *gorm.DB
	hub   Notifier
	delay time.Duration

	// Consultas a la base, reemplazables en los tests.
	tenantOf      func(table string, id uint) uint
	membersOnline func(tenant uint, online []uint) []uint

	mu      sync.Mutex
	pending map[change]bool
	timer   *time.Timer
}

// NewEmitter crea el emisor. db se usa solo para resolver empresas y
// destinatarios, fuera de la transacción que originó el cambio.
func NewEmitter(db *gorm.DB, hub Notifier, delay time.Duration) *Emitter {
	e := &Emitter{db: db, hub: hub, delay: delay, pending: map[change]bool{}}
	e.tenantOf = e.tenantOfDB
	e.membersOnline = e.membersOnlineDB
	return e
}

// Register engancha el emisor a las escrituras de db (crear, actualizar,
// borrar) y lo devuelve.
func Register(db *gorm.DB, hub Notifier) *Emitter {
	e := NewEmitter(db.Session(&gorm.Session{NewDB: true}), hub, 500*time.Millisecond)
	cb := func(tx *gorm.DB) { e.observe(tx) }
	_ = db.Callback().Create().After("gorm:create").Register("realtime:create", cb)
	_ = db.Callback().Update().After("gorm:update").Register("realtime:update", cb)
	_ = db.Callback().Delete().After("gorm:delete").Register("realtime:delete", cb)
	return e
}

// observe mira una escritura recién hecha y, si es de una tabla vigilada,
// anota el cambio. Barato: nada de consultas aquí, va dentro de la escritura.
func (e *Emitter) observe(tx *gorm.DB) {
	st := tx.Statement
	if tx.Error != nil || st == nil || st.Schema == nil {
		return
	}
	w, ok := watched[st.Schema.Table]
	if !ok {
		return
	}
	c := change{entity: w.entity}

	rv := reflect.Indirect(st.ReflectValue)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		if rv.Len() == 0 {
			e.add(c)
			return
		}
		rv = reflect.Indirect(rv.Index(0))
	}
	if rv.Kind() == reflect.Struct {
		ctx := st.Context
		if ctx == nil {
			ctx = context.Background()
		}
		if f := st.Schema.LookUpField("TenantID"); f != nil {
			if v, zero := f.ValueOf(ctx, rv); !zero {
				c.tenantID = toUint(v)
			}
		}
		if c.tenantID == 0 {
			switch {
			case w.parentTable != "":
				if f := st.Schema.LookUpField(w.parentField); f != nil {
					if v, zero := f.ValueOf(ctx, rv); !zero {
						c.lookupTable, c.lookupID = w.parentTable, toUint(v)
					}
				}
			case st.Schema.LookUpField("TenantID") != nil:
				// La fila tiene tenant_id pero el struct no lo traía (p. ej.
				// Model(&Task{ID: 5}).Updates(...)): se busca por su id.
				if f := st.Schema.LookUpField("ID"); f != nil {
					if v, zero := f.ValueOf(ctx, rv); !zero {
						c.lookupTable, c.lookupID = st.Schema.Table, toUint(v)
					}
				}
			}
		}
		// Una jornada dice de quién es: así cada cliente sabe si le toca
		// recargar sin pedir nada.
		if w.entity == EntityWorkHour {
			if f := st.Schema.LookUpField("UserID"); f != nil {
				if v, zero := f.ValueOf(ctx, rv); !zero {
					c.subject = toUint(v)
				}
			}
		}
		// Un empleo dice su empresa en company_id.
		if c.tenantID == 0 && st.Schema.Table == "employments" {
			if f := st.Schema.LookUpField("CompanyID"); f != nil {
				if v, zero := f.ValueOf(ctx, rv); !zero {
					c.tenantID = toUint(v)
				}
			}
		}
	}
	e.add(c)
}

func (e *Emitter) add(c change) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pending[c] = true
	if e.timer == nil {
		e.timer = time.AfterFunc(e.delay, e.flush)
	}
}

// flush despacha lo acumulado: un aviso por (entidad, empresa) a cada usuario
// conectado de esa empresa. Si la empresa no se pudo saber, avisa a todos los
// conectados: el aviso no lleva datos y cada uno recarga con sus permisos.
func (e *Emitter) flush() {
	e.mu.Lock()
	batch := e.pending
	e.pending = map[change]bool{}
	e.timer = nil
	e.mu.Unlock()

	online := e.hub.OnlineUserIDs()
	if len(online) == 0 {
		return
	}

	type key struct {
		entity string
		tenant uint
	}
	// Por (entidad, empresa): de quiénes son las jornadas que cambiaron. Si
	// alguna no se sabe, el aviso no lleva user_ids y recargan todos.
	type group struct {
		subjects map[uint]bool
		unknown  bool
	}
	groups := map[key]*group{}
	var order []key
	for c := range batch {
		tenant := c.tenantID
		if tenant == 0 && c.lookupTable != "" && c.lookupID != 0 {
			tenant = e.tenantOf(c.lookupTable, c.lookupID)
		}
		k := key{c.entity, tenant}
		g := groups[k]
		if g == nil {
			g = &group{subjects: map[uint]bool{}}
			groups[k] = g
			order = append(order, k)
		}
		if c.subject != 0 {
			g.subjects[c.subject] = true
		} else {
			g.unknown = true
		}
	}

	for _, k := range order {
		g := groups[k]
		recipients := online
		if k.tenant != 0 {
			recipients = e.membersOnline(k.tenant, online)
		}
		payload := map[string]interface{}{"entity": k.entity}
		if k.tenant != 0 {
			payload["tenant_id"] = k.tenant
		}
		if k.entity == EntityWorkHour && !g.unknown && len(g.subjects) > 0 {
			ids := make([]uint, 0, len(g.subjects))
			for id := range g.subjects {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			payload["user_ids"] = ids
		}
		for _, uid := range recipients {
			e.hub.NotifyUser(uid, EventType, payload)
		}
	}
}

func (e *Emitter) tenantOfDB(table string, id uint) uint {
	if e.db == nil {
		return 0
	}
	var tenant uint
	if err := e.db.Table(table).Select("tenant_id").Where("id = ?", id).
		Limit(1).Scan(&tenant).Error; err != nil {
		return 0
	}
	return tenant
}

// membersOnlineDB filtra a los conectados que trabajan en la empresa: la propia
// empresa, quien tiene un empleo activo en ella o la tiene como empleador, y
// el equipo de plataforma (superadmin y Customer Success), que mira cualquiera.
func (e *Emitter) membersOnlineDB(tenant uint, online []uint) []uint {
	if e.db == nil {
		return online
	}
	var ids []uint
	err := e.db.Raw(`
		SELECT u.id FROM users u
		WHERE u.id IN ? AND u.deleted_at IS NULL AND (
			u.id = ? OR u.empleador_id = ?
			OR u.is_superadmin = true
			OR u.user_type IN ('superadmin', 'customer_success')
			OR EXISTS (SELECT 1 FROM employments em
				WHERE em.user_id = u.id AND em.company_id = ?
				  AND em.status = 'active' AND em.deleted_at IS NULL)
		)`, online, tenant, tenant, tenant).Scan(&ids).Error
	if err != nil {
		// Mejor un aviso de más que dejar a todos sin enterarse.
		return online
	}
	return ids
}

func toUint(v interface{}) uint {
	switch n := v.(type) {
	case uint:
		return n
	case uint64:
		return uint(n)
	case uint32:
		return uint(n)
	case int:
		if n > 0 {
			return uint(n)
		}
	case int64:
		if n > 0 {
			return uint(n)
		}
	case *uint:
		if n != nil {
			return *n
		}
	}
	return 0
}
