package main

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// resetDemoData borra TODO lo sembrado y nada más.
//
// El criterio es único y verificable: una fila es de demo si pertenece a un
// usuario cuyo correo termina en demoDomain, o a la empresa de uno de ellos.
// Por eso el seeder no crea nada fuera de ese dominio: es lo que hace que este
// borrado sea seguro de correr en una base con datos reales al lado.
func resetDemoData(db *gorm.DB) error {
	users := fmt.Sprintf("(SELECT id FROM users WHERE email LIKE '%%@%s')", demoDomain)
	tenants := fmt.Sprintf("(SELECT id FROM users WHERE email LIKE '%%@%s' AND user_type = 'empleador')", demoDomain)

	boards := fmt.Sprintf("(SELECT id FROM boards WHERE tenant_id IN %s OR created_by IN %s)", tenants, users)
	tasks := fmt.Sprintf("(SELECT id FROM tasks WHERE tenant_id IN %s OR created_by IN %s)", tenants, users)
	channels := fmt.Sprintf("(SELECT id FROM channels WHERE tenant_id IN %s OR created_by IN %s)", tenants, users)
	messages := fmt.Sprintf("(SELECT id FROM channel_messages WHERE channel_id IN %s OR user_id IN %s)", channels, users)
	employments := fmt.Sprintf("(SELECT id FROM employments WHERE user_id IN %s OR company_id IN %s)", users, tenants)
	roles := fmt.Sprintf("(SELECT id FROM roles WHERE tenant_id IN %s)", tenants)
	groups := fmt.Sprintf("(SELECT id FROM groups WHERE tenant_id IN %s)", tenants)
	incidents := fmt.Sprintf("(SELECT id FROM incidents WHERE created_by IN %s)", users)

	// El orden va de las hojas a la raíz: la tabla users se borra al final
	// porque todas las subconsultas de arriba dependen de ella.
	steps := []struct {
		table string
		sql   string
	}{
		{"message_reactions", fmt.Sprintf("DELETE FROM message_reactions WHERE user_id IN %s OR message_id IN %s", users, messages)},
		{"starred_messages", fmt.Sprintf("DELETE FROM starred_messages WHERE user_id IN %s OR message_id IN %s", users, messages)},
		{"mentions", fmt.Sprintf("DELETE FROM mentions WHERE user_id IN %s OR message_id IN %s", users, messages)},
		{"support_tickets", fmt.Sprintf("DELETE FROM support_tickets WHERE requester_id IN %s OR channel_id IN %s", users, channels)},
		{"channel_messages", fmt.Sprintf("DELETE FROM channel_messages WHERE channel_id IN %s OR user_id IN %s", channels, users)},
		{"channel_members", fmt.Sprintf("DELETE FROM channel_members WHERE channel_id IN %s OR user_id IN %s", channels, users)},
		{"hidden_channels", fmt.Sprintf("DELETE FROM hidden_channels WHERE channel_id IN %s OR user_id IN %s", channels, users)},
		{"channels", fmt.Sprintf("DELETE FROM channels WHERE id IN %s", channels)},

		{"comments", fmt.Sprintf("DELETE FROM comments WHERE task_id IN %s OR user_id IN %s", tasks, users)},
		{"task_attachments", fmt.Sprintf("DELETE FROM task_attachments WHERE task_id IN %s", tasks)},
		{"task_users", fmt.Sprintf("DELETE FROM task_users WHERE task_id IN %s OR user_id IN %s", tasks, users)},
		{"tasks", fmt.Sprintf("DELETE FROM tasks WHERE id IN %s", tasks)},
		{"board_invitations", fmt.Sprintf("DELETE FROM board_invitations WHERE board_id IN %s OR user_id IN %s", boards, users)},
		{"board_members", fmt.Sprintf("DELETE FROM board_members WHERE board_id IN %s OR user_id IN %s", boards, users)},
		// board_phases sí tiene una FK física hacia phases, así que hay que
		// soltar el vínculo y borrar la fase en la MISMA sentencia. El NOT IN
		// final protege una fase que además cuelgue de un tablero ajeno.
		{"board_phases", fmt.Sprintf(`
			WITH removed AS (
				DELETE FROM board_phases WHERE board_id IN %s RETURNING phase_id
			)
			DELETE FROM phases WHERE id IN (SELECT phase_id FROM removed)
			  AND id NOT IN (SELECT phase_id FROM board_phases WHERE board_id NOT IN %s)`, boards, boards)},
		{"boards", fmt.Sprintf("DELETE FROM boards WHERE id IN %s", boards)},

		{"work_hours", fmt.Sprintf("DELETE FROM work_hours WHERE user_id IN %s OR tenant_id IN %s", users, tenants)},
		{"employment_managers", fmt.Sprintf("DELETE FROM employment_managers WHERE employment_id IN %s OR manager_id IN %s", employments, users)},
		{"employments", fmt.Sprintf("DELETE FROM employments WHERE id IN %s", employments)},

		{"user_roles", fmt.Sprintf("DELETE FROM user_roles WHERE user_id IN %s OR role_id IN %s", users, roles)},
		{"roles", fmt.Sprintf("DELETE FROM roles WHERE id IN %s", roles)},
		{"group_members", fmt.Sprintf("DELETE FROM group_members WHERE user_id IN %s OR group_id IN %s", users, groups)},
		{"groups", fmt.Sprintf("DELETE FROM groups WHERE id IN %s", groups)},

		{"incident_responses", fmt.Sprintf("DELETE FROM incident_responses WHERE incident_id IN %s OR user_id IN %s", incidents, users)},
		{"incidents", fmt.Sprintf("DELETE FROM incidents WHERE id IN %s", incidents)},

		{"notifications", fmt.Sprintf("DELETE FROM notifications WHERE user_id IN %s", users)},
		{"messages", fmt.Sprintf("DELETE FROM messages WHERE user_id IN %s OR tenant_id IN %s", users, tenants)},
		{"audit_logs", fmt.Sprintf("DELETE FROM audit_logs WHERE actor_id IN %s", users)},

		{"users", fmt.Sprintf("DELETE FROM users WHERE id IN %s", users)},
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, step := range steps {
			// Una base recién creada (o una rama vieja) puede no tener todas las
			// tablas; que falte una no es motivo para abortar el borrado.
			if !tx.Migrator().HasTable(step.table) {
				continue
			}
			res := tx.Exec(step.sql)
			if res.Error != nil {
				return fmt.Errorf("%s: %w", step.table, res.Error)
			}
			if res.RowsAffected > 0 {
				log.Printf("  - %s: %d filas", step.table, res.RowsAffected)
			}
		}
		return nil
	})
}
