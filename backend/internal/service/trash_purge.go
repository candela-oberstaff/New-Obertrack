package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Borrado en firme desde la Papelera.
//
// El "eliminar para siempre" era un DELETE de la fila y nada más, y la base lo
// rechazaba por clave foránea en cuanto la fila tenía algo colgando: 44 de 47
// fallaban con un "quizás tienen datos vinculados". Lo que cuelga es distinto
// por tipo, y eso decide qué es borrar de verdad:
//
//   - Una TAREA arrastra solo lo suyo (asignados, comentarios, adjuntos,
//     historial, enlaces de calendario, ejecuciones de automatización). Se va
//     entera.
//   - Un TABLERO arrastra lo suyo, sus tareas y sus automatizaciones. Cascada.
//   - Un USUARIO arrastra historial DE OTROS: horas que la empresa aprobó,
//     tareas que creó, mensajes, tickets. Eso no se destruye. Se borra del todo
//     solo si no deja nada; si deja historial, se ANONIMIZA: correo liberado,
//     datos personales fuera, y las horas y tareas quedan a nombre de
//     "Usuario eliminado". Es lo que la Papelera prometía —que el correo quede
//     libre, que la persona desaparezca— sin romper el expediente de un cliente.

// PurgeOutcome dice qué pasó de verdad, porque no es lo mismo y quien pulsa
// el botón tiene que saberlo.
type PurgeOutcome string

const (
	PurgeDeleted    PurgeOutcome = "deleted"
	PurgeAnonymized PurgeOutcome = "anonymized"
)

// purgeTask borra una tarea y todo lo que cuelga de ella. Sirve suelta y desde
// la cascada del tablero, y por eso recibe la transacción.
func purgeTask(tx *gorm.DB, id uint) error {
	stmts := []string{
		`DELETE FROM task_users WHERE task_id = ?`,
		`DELETE FROM comments WHERE task_id = ?`,
		`DELETE FROM task_attachments WHERE task_id = ?`,
		`DELETE FROM task_status_history WHERE task_id = ?`,
		`DELETE FROM calendar_sync_jobs WHERE task_id = ?`,
		`DELETE FROM calendar_event_links WHERE task_id = ?`,
		// Una reunión es un registro de que ocurrió; se desliga, no se borra.
		`UPDATE meeting_sessions SET task_id = NULL WHERE task_id = ?`,
		`DELETE FROM workflow_step_runs WHERE run_id IN (SELECT id FROM workflow_runs WHERE entity_type = 'task' AND entity_id = ?)`,
		`DELETE FROM workflow_runs WHERE entity_type = 'task' AND entity_id = ?`,
		`DELETE FROM tasks WHERE id = ?`,
	}
	for _, q := range stmts {
		if err := tx.Exec(q, id).Error; err != nil {
			return err
		}
	}
	return nil
}

// purgeBoard borra el tablero, sus tareas (estén o no en la Papelera: el
// tablero se va, sus tareas no tienen dónde vivir) y sus automatizaciones.
func purgeBoard(tx *gorm.DB, id uint) error {
	var taskIDs []uint
	if err := tx.Raw(`SELECT id FROM tasks WHERE board_id = ?`, id).Scan(&taskIDs).Error; err != nil {
		return err
	}
	for _, tid := range taskIDs {
		if err := purgeTask(tx, tid); err != nil {
			return err
		}
	}
	stmts := []string{
		`DELETE FROM workflow_step_runs WHERE run_id IN (SELECT r.id FROM workflow_runs r JOIN workflows w ON w.id = r.workflow_id WHERE w.board_id = ?)`,
		`DELETE FROM workflow_runs WHERE workflow_id IN (SELECT id FROM workflows WHERE board_id = ?)`,
		`DELETE FROM workflow_steps WHERE workflow_id IN (SELECT id FROM workflows WHERE board_id = ?)`,
		`DELETE FROM workflows WHERE board_id = ?`,
		`DELETE FROM board_invitations WHERE board_id = ?`,
		`DELETE FROM board_members WHERE board_id = ?`,
		`DELETE FROM board_phases WHERE board_id = ?`,
		`DELETE FROM boards WHERE id = ?`,
	}
	for _, q := range stmts {
		if err := tx.Exec(q, id).Error; err != nil {
			return err
		}
	}
	return nil
}

// userLeafTables es lo que es SOLO de la persona y no cuenta la historia de
// nadie más: se borra siempre, tanto si después la fila se va como si se
// anonimiza. Lo que no está aquí (horas, tareas creadas, mensajes, tickets,
// empleos con su expediente) es historial compartido y se conserva.
var userLeafStatements = []string{
	`DELETE FROM notifications WHERE user_id = ?`,
	`DELETE FROM channel_members WHERE user_id = ?`,
	`DELETE FROM board_members WHERE user_id = ?`,
	`DELETE FROM task_users WHERE user_id = ?`,
	`DELETE FROM message_reactions WHERE user_id = ?`,
	`DELETE FROM survey_responses WHERE user_id = ?`,
	`DELETE FROM mass_email_logs WHERE user_id = ?`,
	`DELETE FROM user_activity_daily WHERE user_id = ?`,
	`DELETE FROM contact_logs WHERE user_id = ?`,
}

// purgeUser intenta el borrado total y, si la persona deja historial, la
// anonimiza. Las dos cosas dentro de la misma transacción: el intento fallido
// de DELETE se deshace en un savepoint y no ensucia el resto.
func purgeUser(tx *gorm.DB, id uint) (PurgeOutcome, error) {
	for _, q := range userLeafStatements {
		if err := tx.Exec(q, id).Error; err != nil {
			return "", err
		}
	}

	// Intento de borrado total en un savepoint: si la base lo rechaza por una
	// clave foránea, es que hay historial, y se sigue por la anonimización.
	if err := tx.SavePoint("sp_purge_user").Error; err != nil {
		return "", err
	}
	if err := tx.Exec(`DELETE FROM users WHERE id = ?`, id).Error; err == nil {
		return PurgeDeleted, nil
	} else if !isForeignKeyViolation(err) {
		return "", err
	}
	if err := tx.RollbackTo("sp_purge_user").Error; err != nil {
		return "", err
	}

	// Anonimizar: el correo queda libre (es lo que la Papelera promete al
	// decir "para liberar el correo"), el dato personal desaparece, y la fila
	// sigue existiendo para que las horas y tareas tengan a quién apuntar.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(raw)), bcrypt.MinCost)
	if err != nil {
		return "", err
	}
	res := tx.Exec(`
		UPDATE users SET
			name = 'Usuario eliminado',
			email = ?,
			password = ?,
			phone_number = '', address = '', city = '', state = '', country = '', location = '',
			identity_document = '', emergency_phones = '', birth_date = NULL,
			avatar = '', obersuite_id = '',
			is_active = false,
			purged_at = NOW(),
			updated_at = NOW()
		WHERE id = ?`,
		fmt.Sprintf("eliminado-%d@borrado.obertrack.invalid", id), string(hash), id,
	)
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "", fmt.Errorf("el usuario %d no existe", id)
	}
	return PurgeAnonymized, nil
}

// isForeignKeyViolation reconoce el rechazo por clave foránea (SQLSTATE 23503)
// por texto, como el resto del código, para no atarse al driver.
func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "23503") || strings.Contains(msg, "foreign key")
}
