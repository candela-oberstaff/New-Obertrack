package realtime

import (
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/obertrack/backend/internal/models"
)

type sent struct {
	userID uint
	data   map[string]interface{}
}

type fakeHub struct {
	mu     sync.Mutex
	online []uint
	got    []sent
}

func (h *fakeHub) OnlineUserIDs() []uint { return h.online }

func (h *fakeHub) NotifyUser(userID uint, notifType string, data interface{}) {
	if notifType != EventType {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.got = append(h.got, sent{userID, data.(map[string]interface{})})
}

func (h *fakeHub) events() []sent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]sent(nil), h.got...)
}

// dryDB arma el SQL sin conectarse: los callbacks corren igual, que es lo que
// se prueba aquí.
func dryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 user=x dbname=x sslmode=disable"}),
		&gorm.Config{DryRun: true, SkipDefaultTransaction: true, DisableAutomaticPing: true, Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// setup engancha un emisor con hub falso y consultas simuladas: la empresa 7
// tiene a los usuarios 1 y 2; el 3 es de otra.
func setup(t *testing.T) (*gorm.DB, *fakeHub, *Emitter) {
	t.Helper()
	db := dryDB(t)
	hub := &fakeHub{online: []uint{1, 2, 3}}
	e := Register(db, hub)
	e.delay = 20 * time.Millisecond
	e.tenantOf = func(table string, id uint) uint {
		if (table == "tasks" && id == 50) || (table == "boards" && id == 9) {
			return 7
		}
		return 0
	}
	e.membersOnline = func(tenant uint, online []uint) []uint {
		if tenant == 7 {
			return []uint{1, 2}
		}
		return nil
	}
	return db, hub, e
}

func waitEvents(t *testing.T, hub *fakeHub) []sent {
	t.Helper()
	time.Sleep(120 * time.Millisecond)
	return hub.events()
}

func TestCrearTareaAvisaSoloASuEmpresa(t *testing.T) {
	db, hub, _ := setup(t)
	db.Create(&models.Task{Title: "x", TenantID: 7, BoardID: 9})

	got := waitEvents(t, hub)
	if len(got) != 2 {
		t.Fatalf("avisos = %+v, esperaba 2 (usuarios 1 y 2)", got)
	}
	for _, s := range got {
		if s.userID == 3 {
			t.Fatal("el usuario 3 es de otra empresa y no debería enterarse")
		}
		if s.data["entity"] != EntityTask || s.data["tenant_id"] != uint(7) {
			t.Fatalf("payload = %v", s.data)
		}
	}
}

func TestComentarioBuscaLaEmpresaPorSuTarea(t *testing.T) {
	db, hub, _ := setup(t)
	db.Create(&models.Comment{TaskID: 50, UserID: 1, Content: "hola"})

	got := waitEvents(t, hub)
	if len(got) != 2 || got[0].data["entity"] != EntityTask {
		t.Fatalf("avisos = %+v, esperaba 2 de task", got)
	}
}

func TestMiembroDeTableroAvisaBoard(t *testing.T) {
	db, hub, _ := setup(t)
	db.Create(&models.BoardMember{BoardID: 9, UserID: 2})

	got := waitEvents(t, hub)
	if len(got) != 2 || got[0].data["entity"] != EntityBoard {
		t.Fatalf("avisos = %+v, esperaba 2 de board", got)
	}
}

func TestUnaRafagaEsUnSoloAviso(t *testing.T) {
	db, hub, _ := setup(t)
	for i := 0; i < 10; i++ {
		db.Create(&models.WorkHour{UserID: 1, TenantID: 7})
	}
	got := waitEvents(t, hub)
	// Diez jornadas de la misma empresa: un aviso por usuario, no diez.
	if len(got) != 2 {
		t.Fatalf("avisos = %d, esperaba 2", len(got))
	}
	if got[0].data["entity"] != EntityWorkHour {
		t.Fatalf("payload = %v", got[0].data)
	}
}

func TestSinEmpresaAvisaATodosLosConectados(t *testing.T) {
	db, hub, _ := setup(t)
	// Un borrado por condición no trae fila: no se sabe la empresa.
	db.Where("id = ?", 99).Delete(&models.Task{})
	got := waitEvents(t, hub)
	if len(got) != 3 {
		t.Fatalf("avisos = %d, esperaba 3 (todos los conectados)", len(got))
	}
	if _, has := got[0].data["tenant_id"]; has {
		t.Fatalf("sin empresa el payload no debería traer tenant_id: %v", got[0].data)
	}
}

func TestTablaNoVigiladaNoAvisa(t *testing.T) {
	db, hub, _ := setup(t)
	db.Create(&models.Notification{UserID: 1, Title: "x"})
	if got := waitEvents(t, hub); len(got) != 0 {
		t.Fatalf("avisos = %+v, esperaba ninguno", got)
	}
}

func TestSinNadieConectadoNoConsulta(t *testing.T) {
	db, hub, e := setup(t)
	hub.online = nil
	called := false
	e.membersOnline = func(uint, []uint) []uint { called = true; return nil }
	db.Create(&models.Task{Title: "x", TenantID: 7})
	waitEvents(t, hub)
	if called {
		t.Fatal("sin conectados no hace falta buscar destinatarios")
	}
}
