package migrations

import (
	"fmt"
	"log"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

func Run(db *gorm.DB) error {
	// Ensure the migrations table uses VARCHAR for its ID column.
	// If the table was previously created with an integer id (which overflows
	// for timestamp-style IDs like "202603251200"), drop it so gormigrate
	// can recreate it with the correct type.
	var colType string
	row := db.Raw(`
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'migrations' AND column_name = 'id'
		LIMIT 1
	`).Row()
	if err := row.Scan(&colType); err == nil && colType == "integer" {
		log.Println("Dropping incompatible migrations table (integer id column)...")
		if err := db.Exec(`DROP TABLE IF EXISTS migrations`).Error; err != nil {
			return fmt.Errorf("failed to drop migrations table: %w", err)
		}
	}

	options := &gormigrate.Options{
		TableName:                 "migrations",
		IDColumnName:              "id",
		IDColumnSize:              255,
		UseTransaction:            false,
		ValidateUnknownMigrations: false,
	}
	m := gormigrate.New(db, options, []*gormigrate.Migration{
		// Initial migration: create all existing tables
		{
			ID: "202603251200", // Current date/time as ID
			Migrate: func(tx *gorm.DB) error {
				err := tx.AutoMigrate(
					&models.User{},
					&models.Board{},
					&models.Task{},
					&models.TaskUser{},
					&models.Comment{},
					&models.TaskAttachment{},
					&models.WorkHour{},
					&models.Notification{},
					&models.MassEmailLog{},
					&models.Message{},
					&models.Channel{},
					&models.ChannelMember{},
					&models.ChannelMessage{},
					&models.MessageReaction{},
					&models.StarredMessage{},
					&models.UserStatus{},
					&models.Mention{},
				)
				if err != nil {
					return err
				}

				// Create board_members manually to defer constraints or handle separately if AutoMigrate fails
				if err := tx.AutoMigrate(&models.BoardMember{}); err != nil {
					log.Printf("Warning: initial board_members migrate failed: %v", err)
				}
				return nil
			},
		},
		{
			ID: "202603251500_add_task_attachments",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.TaskAttachment{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&models.TaskAttachment{})
			},
		},
		{
			ID: "202604161345_fix_task_null_constraints",
			Migrate: func(tx *gorm.DB) error {
				// Make start_date, end_date and order nullable to match current application logic
				sql := `
					ALTER TABLE tasks ALTER COLUMN start_date DROP NOT NULL;
					ALTER TABLE tasks ALTER COLUMN end_date DROP NOT NULL;
					ALTER TABLE tasks ALTER COLUMN "order" DROP NOT NULL;
				`
				return tx.Exec(sql).Error
			},
		},
		{
			ID: "202605131730_reset_email_tables_clean",
			Migrate: func(tx *gorm.DB) error {
				// Drop existing tables to avoid conflicts with old schemas
				tx.Migrator().DropTable("email_campaigns", "email_templates")
				return tx.AutoMigrate(&models.EmailTemplate{}, &models.EmailCampaign{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&models.EmailTemplate{}, &models.EmailCampaign{})
			},
		},
		{
			ID: "202605141135_add_email_recipient_list",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.EmailCampaign{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&models.EmailCampaign{}, "recipient_list")
			},
		},
		{
			ID: "202605141615_add_survey_tables",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(
					&models.Survey{},
					&models.SurveyQuestion{},
					&models.SurveyResponse{},
					&models.SurveyAnswer{},
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(
					&models.Survey{},
					&models.SurveyQuestion{},
					&models.SurveyResponse{},
					&models.SurveyAnswer{},
				)
			},
		},
		{
			ID: "202605151120_add_email_tracking",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.EmailCampaign{}, &models.EmailEvent{})
			},
			Rollback: func(tx *gorm.DB) error {
				tx.Migrator().DropColumn(&models.EmailCampaign{}, "brevo_campaign_id")
				return tx.Migrator().DropTable(&models.EmailEvent{})
			},
		},
		{
			ID: "202605181130_fix_duplicate_columns_and_tables",
			Migrate: func(tx *gorm.DB) error {
				// 1. Migrate users table (tipo_usuario -> user_type)
				if tx.Migrator().HasColumn(&models.User{}, "tipo_usuario") {
					log.Println("Migrating user_type data and dropping tipo_usuario...")
					// Ensure user_type column exists
					if !tx.Migrator().HasColumn(&models.User{}, "user_type") {
						if err := tx.Migrator().AddColumn(&models.User{}, "user_type"); err != nil {
							return err
						}
					}
					// Copy and map values (Gorm was writing to tipo_usuario, so it is the source of truth for new users)
					updateSQL := `
						UPDATE users 
						SET user_type = CASE 
							WHEN tipo_usuario = 'empleado' THEN 'profesional'
							WHEN tipo_usuario = 'empleador' THEN 'empleador'
							WHEN tipo_usuario = 'superadmin' THEN 'superadmin'
							ELSE 'profesional'
						END 
						WHERE tipo_usuario IS NOT NULL AND (user_type IS NULL OR user_type = '')
					`
					if err := tx.Exec(updateSQL).Error; err != nil {
						return err
					}
					// Drop column tipo_usuario
					if err := tx.Migrator().DropColumn(&models.User{}, "tipo_usuario"); err != nil {
						return err
					}
				}

				// 2. Migrate task_attachments table (filename -> file_name, stored_filename -> file_url)
				if tx.Migrator().HasColumn(&models.TaskAttachment{}, "filename") {
					log.Println("Migrating task_attachments filename -> file_name...")
					if !tx.Migrator().HasColumn(&models.TaskAttachment{}, "file_name") {
						if err := tx.Migrator().AddColumn(&models.TaskAttachment{}, "file_name"); err != nil {
							return err
						}
					}
					updateNameSQL := `UPDATE task_attachments SET file_name = filename WHERE filename IS NOT NULL AND (file_name IS NULL OR file_name = '')`
					if err := tx.Exec(updateNameSQL).Error; err != nil {
						return err
					}
				}
				if tx.Migrator().HasColumn(&models.TaskAttachment{}, "stored_filename") {
					log.Println("Migrating task_attachments stored_filename -> file_url...")
					if !tx.Migrator().HasColumn(&models.TaskAttachment{}, "file_url") {
						if err := tx.Migrator().AddColumn(&models.TaskAttachment{}, "file_url"); err != nil {
							return err
						}
					}
					updateUrlSQL := `UPDATE task_attachments SET file_url = stored_filename WHERE stored_filename IS NOT NULL AND (file_url IS NULL OR file_url = '')`
					if err := tx.Exec(updateUrlSQL).Error; err != nil {
						return err
					}
				}
				// Now drop old columns filename and stored_filename
				if tx.Migrator().HasColumn(&models.TaskAttachment{}, "filename") {
					if err := tx.Migrator().DropColumn(&models.TaskAttachment{}, "filename"); err != nil {
						return err
					}
				}
				if tx.Migrator().HasColumn(&models.TaskAttachment{}, "stored_filename") {
					if err := tx.Migrator().DropColumn(&models.TaskAttachment{}, "stored_filename"); err != nil {
						return err
					}
				}

				// 3. Migrate task_user -> task_users table
				if tx.Migrator().HasTable("task_user") {
					log.Println("Migrating task_user -> task_users...")
					// Ensure task_users table exists
					if err := tx.AutoMigrate(&models.TaskUser{}); err != nil {
						return err
					}

					// Copy rows
					copySQL := `
						INSERT INTO task_users (task_id, user_id) 
						SELECT task_id, user_id FROM task_user
						ON CONFLICT DO NOTHING
					`
					if err := tx.Exec(copySQL).Error; err != nil {
						return err
					}

					// Drop old table
					if err := tx.Migrator().DropTable("task_user"); err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "20260518_add_user_reset_token",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Migrating reset_token fields to users...")
				return tx.AutoMigrate(&models.User{})
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&models.User{}, "reset_token") {
					tx.Migrator().DropColumn(&models.User{}, "reset_token")
				}
				if tx.Migrator().HasColumn(&models.User{}, "reset_token_expiry") {
					tx.Migrator().DropColumn(&models.User{}, "reset_token_expiry")
				}
				return nil
			},
		},
		{
			ID: "20260525_add_ticket_system",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creating ticket system tables (contacts, tickets, ticket_messages)...")
				return tx.AutoMigrate(
					&models.Contact{},
					&models.Ticket{},
					&models.TicketMessage{},
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(
					"ticket_messages",
					"tickets",
					"contacts",
				)
			},
		},
		{
			ID: "202605271200_add_tutorials",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creating tutorials table...")
				return tx.AutoMigrate(&models.Tutorial{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&models.Tutorial{})
			},
		},
		{
			ID: "202605271600_add_tutorial_category_and_views",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding category to tutorials and creating tutorial_views...")
				if err := tx.AutoMigrate(&models.Tutorial{}); err != nil {
					return err
				}
				return tx.AutoMigrate(&models.TutorialView{})
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Migrator().DropTable(&models.TutorialView{}); err != nil {
					return err
				}
				return tx.Migrator().DropColumn(&models.Tutorial{}, "category")
			},
		},
		{
			ID: "202605291200_add_tenant_id",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding tenant_id to boards, tasks, work_hours, channels, channel_messages and messages...")

				if tx.Migrator().HasColumn(&models.Message{}, "company_id") && !tx.Migrator().HasColumn(&models.Message{}, "tenant_id") {
					if err := tx.Migrator().RenameColumn(&models.Message{}, "company_id", "tenant_id"); err != nil {
						return err
					}
				}

				// Check for board_members and channel_members constraint issue: orphaned rows exist
				// Ensure we don't have members with user_id that doesn't exist in users
				if tx.Migrator().HasTable("board_members") {
					log.Println("Cleaning up orphaned board_members before migration...")
					tx.Exec(`DELETE FROM board_members WHERE user_id NOT IN (SELECT id FROM users)`)
				}
				if tx.Migrator().HasTable("channel_members") {
					log.Println("Cleaning up orphaned channel_members before migration...")
					tx.Exec(`DELETE FROM channel_members WHERE user_id NOT IN (SELECT id FROM users)`)
				}

				if err := tx.AutoMigrate(
					&models.Board{},
					&models.Task{},
					&models.WorkHour{},
					&models.Channel{},
					&models.ChannelMessage{},
					&models.Message{},
				); err != nil {
					return err
				}

				tenantExpr := "(SELECT CASE WHEN u.user_type = 'empleador' THEN u.id ELSE u.empleador_id END FROM users u WHERE u.id = %s)"

				// COALESCE(..., tenant_id): si el creador no produce un tenant (usuario
				// borrado, profesional sin empleador, superadmin), la fila conserva su
				// valor actual en vez de violar NOT NULL en bases donde la columna ya
				// existe con esa restricción. La migración 202605291400 les asigna el
				// tenant de fallback.
				statements := []string{
					fmt.Sprintf("UPDATE boards SET tenant_id = COALESCE("+tenantExpr+", tenant_id) WHERE tenant_id IS NULL OR tenant_id = 0", "boards.created_by"),
					"UPDATE tasks SET tenant_id = COALESCE((SELECT b.tenant_id FROM boards b WHERE b.id = tasks.board_id), tenant_id) WHERE tenant_id IS NULL OR tenant_id = 0",
					fmt.Sprintf("UPDATE channels SET tenant_id = COALESCE("+tenantExpr+", tenant_id) WHERE tenant_id IS NULL OR tenant_id = 0", "channels.created_by"),
					"UPDATE channel_messages SET tenant_id = COALESCE((SELECT c.tenant_id FROM channels c WHERE c.id = channel_messages.channel_id), tenant_id) WHERE tenant_id IS NULL OR tenant_id = 0",
					fmt.Sprintf("UPDATE work_hours SET tenant_id = COALESCE("+tenantExpr+", tenant_id) WHERE tenant_id IS NULL OR tenant_id = 0", "work_hours.user_id"),
					fmt.Sprintf("UPDATE messages SET tenant_id = COALESCE("+tenantExpr+", tenant_id) WHERE tenant_id IS NULL", "messages.user_id"),
				}

				for _, sql := range statements {
					if err := tx.Exec(sql).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				for _, col := range []struct {
					model interface{}
					name  string
				}{
					{&models.Board{}, "tenant_id"},
					{&models.Task{}, "tenant_id"},
					{&models.WorkHour{}, "tenant_id"},
					{&models.Channel{}, "tenant_id"},
					{&models.ChannelMessage{}, "tenant_id"},
				} {
					if tx.Migrator().HasColumn(col.model, col.name) {
						if err := tx.Migrator().DropColumn(col.model, col.name); err != nil {
							return err
						}
					}
				}
				if tx.Migrator().HasColumn(&models.Message{}, "tenant_id") {
					return tx.Migrator().RenameColumn(&models.Message{}, "tenant_id", "company_id")
				}
				return nil
			},
		},
		{
			ID: "202605291400_tenant_id_not_null",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Setting tenant_id NOT NULL on core tenant tables...")

				// Ensure no null values exist before setting NOT NULL
				// We use a safe fallback (1 or first superadmin) if they are still null
				fallbackTenantID := 1
				var firstUser models.User
				if err := tx.Where("user_type = ?", "superadmin").First(&firstUser).Error; err == nil {
					fallbackTenantID = int(firstUser.ID)
				}

				tables := []string{"boards", "tasks", "work_hours", "channels", "channel_messages"}
				for _, t := range tables {
					updateSQL := fmt.Sprintf("UPDATE %s SET tenant_id = %d WHERE tenant_id IS NULL OR tenant_id = 0", t, fallbackTenantID)
					tx.Exec(updateSQL)

					if err := tx.Exec(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN tenant_id SET NOT NULL", t)).Error; err != nil {
						log.Printf("Warning: could not set NOT NULL on %s.tenant_id: %v", t, err)
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				for _, t := range []string{"boards", "tasks", "work_hours", "channels", "channel_messages"} {
					if err := tx.Exec(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN tenant_id DROP NOT NULL", t)).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			ID: "202605301200_add_user_token_version",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding token_version to users (session revocation)...")
				return tx.AutoMigrate(&models.User{})
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&models.User{}, "token_version") {
					return tx.Migrator().DropColumn(&models.User{}, "token_version")
				}
				return nil
			},
		},
		{
			ID: "202606051820_add_work_hour_rejection_fields",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding rejection fields to work_hours...")
				return tx.AutoMigrate(&models.WorkHour{})
			},
			Rollback: func(tx *gorm.DB) error {
				for _, column := range []string{"rejected", "rejected_by", "rejected_at", "rejection_reason"} {
					if tx.Migrator().HasColumn(&models.WorkHour{}, column) {
						if err := tx.Migrator().DropColumn(&models.WorkHour{}, column); err != nil {
							return err
						}
					}
				}
				return nil
			},
		},
		{
			ID: "202606081200_add_user_industry",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding industry to users...")
				return tx.AutoMigrate(&models.User{})
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&models.User{}, "industry") {
					return tx.Migrator().DropColumn(&models.User{}, "industry")
				}
				return nil
			},
		},
		{
			ID: "202606081300_add_user_address",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding address to users...")
				return tx.AutoMigrate(&models.User{})
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&models.User{}, "address") {
					return tx.Migrator().DropColumn(&models.User{}, "address")
				}
				return nil
			},
		},
		{
			ID: "202606081400_add_user_state",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding state to users...")
				return tx.AutoMigrate(&models.User{})
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&models.User{}, "state") {
					return tx.Migrator().DropColumn(&models.User{}, "state")
				}
				return nil
			},
		},
		{
			ID: "202606081500_add_ticket_origin",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding origin/description/user_id to tickets and making contact_id nullable...")
				if err := tx.AutoMigrate(&models.Ticket{}); err != nil {
					return err
				}
				// AutoMigrate does not drop the existing NOT NULL on contact_id.
				if err := tx.Exec(`ALTER TABLE tickets ALTER COLUMN contact_id DROP NOT NULL`).Error; err != nil {
					return err
				}
				// Backfill origin for pre-existing local tickets so they are not
				// mistaken for internal Obertrack alerts on the support board.
				return tx.Exec(`UPDATE tickets SET origin = 'whatsapp' WHERE origin IS NULL OR origin = ''`).Error
			},
			Rollback: func(tx *gorm.DB) error {
				for _, col := range []string{"origin", "description", "user_id"} {
					if tx.Migrator().HasColumn(&models.Ticket{}, col) {
						if err := tx.Migrator().DropColumn(&models.Ticket{}, col); err != nil {
							return err
						}
					}
				}
				return nil
			},
		},
		{
			ID: "202606081600_add_ticket_alert_fields",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding internal-alert fields to tickets (professional_email, company_name, rejected_by_name, reason, work_dates)...")
				return tx.AutoMigrate(&models.Ticket{})
			},
			Rollback: func(tx *gorm.DB) error {
				for _, col := range []string{"professional_email", "company_name", "rejected_by_name", "reason", "work_dates"} {
					if tx.Migrator().HasColumn(&models.Ticket{}, col) {
						if err := tx.Migrator().DropColumn(&models.Ticket{}, col); err != nil {
							return err
						}
					}
				}
				return nil
			},
		},
		{
			ID: "202606081700_add_ticket_professional_phone",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding professional_phone to tickets...")
				return tx.AutoMigrate(&models.Ticket{})
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&models.Ticket{}, "professional_phone") {
					return tx.Migrator().DropColumn(&models.Ticket{}, "professional_phone")
				}
				return nil
			},
		},
		{
			ID: "202606081800_create_ticket_transfers",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creating ticket_transfers audit table...")
				return tx.AutoMigrate(&models.TicketTransfer{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&models.TicketTransfer{})
			},
		},
		{
			ID: "202606081900_create_audit_logs",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creating audit_logs table...")
				return tx.AutoMigrate(&models.AuditLog{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&models.AuditLog{})
			},
		},
		{
			ID: "202606082000_audit_logs_data_fields",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding kind/entity_type/entity_id/changes to audit_logs...")
				if err := tx.AutoMigrate(&models.AuditLog{}); err != nil {
					return err
				}
				// Existing rows are activity events.
				return tx.Exec(`UPDATE audit_logs SET kind = 'activity' WHERE kind IS NULL OR kind = ''`).Error
			},
			Rollback: func(tx *gorm.DB) error {
				for _, col := range []string{"kind", "entity_type", "entity_id", "changes"} {
					if tx.Migrator().HasColumn(&models.AuditLog{}, col) {
						if err := tx.Migrator().DropColumn(&models.AuditLog{}, col); err != nil {
							return err
						}
					}
				}
				return nil
			},
		},
		{
			ID: "202606111200_add_tutorial_audience",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Adding audience to tutorials...")
				if err := tx.AutoMigrate(&models.Tutorial{}); err != nil {
					return err
				}
				// Existing tutorials stay visible for everyone.
				return tx.Exec(`UPDATE tutorials SET audience = 'all' WHERE audience IS NULL OR audience = ''`).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&models.Tutorial{}, "audience")
			},
		},
		{
			ID: "202606121500_add_roles_and_groups",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creating roles, user_roles, groups and group_members tables...")
				return tx.AutoMigrate(
					&models.Role{},
					&models.UserRole{},
					&models.Group{},
					&models.GroupMember{},
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(
					"user_roles",
					"roles",
					"group_members",
					"groups",
				)
			},
		},
		{
			ID: "202606131000_add_inactivity_alerts",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creating inactivity_alerts table...")
				return tx.AutoMigrate(&models.InactivityAlert{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("inactivity_alerts")
			},
		},
		{
			ID: "202606131800_add_follow_ups",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creating follow_ups table (bitácora de gestión CS)...")
				return tx.AutoMigrate(&models.FollowUp{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("follow_ups")
			},
		},
		{
			ID: "202606141200_add_employments",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creating employments table (membresías + expediente)...")
				if err := tx.AutoMigrate(&models.Employment{}); err != nil {
					return err
				}
				// Una sola membresía ACTIVA por (profesional, empresa). Permite
				// re-contrataciones (una activa nueva tras una 'ended' previa).
				if err := tx.Exec(`
					CREATE UNIQUE INDEX IF NOT EXISTS idx_employment_active_unique
					ON employments (user_id, company_id)
					WHERE deleted_at IS NULL AND status = 'active'
				`).Error; err != nil {
					return err
				}
				// Backfill: cada profesional/CS con empleador_id → una membresía
				// activa. Idempotente (NOT EXISTS) por si se re-corre.
				return tx.Exec(`
					INSERT INTO employments (user_id, company_id, job_title, manager_id, status, started_at, created_at, updated_at)
					SELECT u.id, u.empleador_id, COALESCE(u.job_title, ''), u.manager_id, 'active', u.created_at, NOW(), NOW()
					FROM users u
					WHERE u.empleador_id IS NOT NULL
						AND u.deleted_at IS NULL
						AND u.user_type IN ('profesional', 'customer_success')
						AND NOT EXISTS (
							SELECT 1 FROM employments e
							WHERE e.user_id = u.id AND e.company_id = u.empleador_id AND e.deleted_at IS NULL
						)
				`).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("employments")
			},
		},
		{
			ID: "202606141600_work_hours_tenant_unique",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Reemplazando índice único de work_hours por (user_id, tenant_id, work_date)...")
				// Quita el viejo único (user_id, work_date) que impediría a un
				// profesional registrar horas en dos empresas el mismo día.
				if err := tx.Exec(`DROP INDEX IF EXISTS idx_user_date`).Error; err != nil {
					return err
				}
				return tx.Exec(`
					CREATE UNIQUE INDEX IF NOT EXISTS idx_user_tenant_date
					ON work_hours (user_id, tenant_id, work_date)
					WHERE deleted_at IS NULL
				`).Error
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Exec(`DROP INDEX IF EXISTS idx_user_tenant_date`).Error; err != nil {
					return err
				}
				return tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_date ON work_hours (user_id, work_date)`).Error
			},
		},
		{
			// FASE 3: expediente laboral. Tablas para evaluaciones/notas y
			// documentos adjuntos a un empleo. El resumen congelado al salir vive
			// en employments.end_summary (ya existente, se llena al terminar).
			ID: "202606151000_add_expediente",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creando tablas del expediente (employment_notes, employment_documents)...")
				return tx.AutoMigrate(
					&models.EmploymentNote{},
					&models.EmploymentDocument{},
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(
					&models.EmploymentNote{},
					&models.EmploymentDocument{},
				)
			},
		},
		{
			// FASE 3: historial de contactos (email/WhatsApp/chat) en el expediente.
			ID: "202606160900_add_contact_logs",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creando tabla de contactos del expediente (contact_logs)...")
				return tx.AutoMigrate(&models.ContactLog{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&models.ContactLog{})
			},
		},
		{
			// Expediente de la empresa: eventos de ciclo de vida (suspensión/reactivación).
			ID: "202606161100_add_company_events",
			Migrate: func(tx *gorm.DB) error {
				log.Println("Creando tabla de eventos de empresa (company_events)...")
				return tx.AutoMigrate(&models.CompanyEvent{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&models.CompanyEvent{})
			},
		},
		{
			// Documentos del expediente: fecha de vencimiento (contratos/certificados).
			ID: "202606161300_doc_expires_at",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.EmploymentDocument{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&models.EmploymentDocument{}, "expires_at")
			},
		},
		{
            // Dedup de la alerta de vencimiento de documentos.
            ID: "202606161500_doc_expiry_alerted_at",
            Migrate: func(tx *gorm.DB) error {
                return tx.AutoMigrate(&models.EmploymentDocument{})
            },
            Rollback: func(tx *gorm.DB) error {
                return tx.Migrator().DropColumn(&models.EmploymentDocument{}, "expiry_alerted_at")
            },
        },
        {
            ID: "202606131900_create_audience_groups",
            Migrate: func(tx *gorm.DB) error {
                log.Println("Creating audience_groups and audience_group_members tables...")
                return tx.AutoMigrate(&models.AudienceGroup{}, &models.AudienceGroupMember{})
            },
            Rollback: func(tx *gorm.DB) error {
                return tx.Migrator().DropTable(
                    "audience_group_members",
                    "audience_groups",
                )
            },
        },
        {
            // Tickets de soporte: gestión (tomar/reasignar/resolver) sobre los canales de soporte.
            ID: "202606171000_add_support_tickets",
            Migrate: func(tx *gorm.DB) error {
                log.Println("Creating support_tickets table...")
                return tx.AutoMigrate(&models.SupportTicket{})
            },
            Rollback: func(tx *gorm.DB) error {
                return tx.Migrator().DropTable("support_tickets")
            },
        },
        {
            // Backfill: crea un ticket 'open' para los canales de soporte existentes
            // (creados antes de esta funcionalidad) que aún no tienen uno.
            ID: "202606171010_backfill_support_tickets",
            Migrate: func(tx *gorm.DB) error {
                log.Println("Backfilling support_tickets for existing support channels...")
                return tx.Exec(`
                    INSERT INTO support_tickets (channel_id, tenant_id, requester_id, status, created_at, updated_at)
                    SELECT c.id, c.tenant_id, c.created_by, 'open', NOW(), NOW()
                    FROM channels c
                    WHERE c.type = 'private'
                      AND c.name LIKE 'Soporte · %'
                      AND c.deleted_at IS NULL
                      AND NOT EXISTS (SELECT 1 FROM support_tickets st WHERE st.channel_id = c.id)
                `).Error
            },
            Rollback: func(tx *gorm.DB) error {
                return nil
            },
        },
        {
            // Reacciones/stars idempotentes: dedupe de filas existentes + índice
            // único. En BD nueva el tag uniqueIndex ya crea el índice; este paso
            // cubre BDs existentes (dedupea primero y luego crea el índice).
            ID: "202606171100_dedupe_reactions_stars_unique",
            Migrate: func(tx *gorm.DB) error {
                log.Println("Deduping message_reactions/starred_messages and creating unique indexes...")
                // 1) Borra duplicados dejando el de menor id en cada grupo.
                if err := tx.Exec(`
                    DELETE FROM message_reactions a
                    USING message_reactions b
                    WHERE a.message_id = b.message_id
                      AND a.user_id = b.user_id
                      AND a.emoji = b.emoji
                      AND a.id > b.id
                `).Error; err != nil {
                    return err
                }
                if err := tx.Exec(`
                    DELETE FROM starred_messages a
                    USING starred_messages b
                    WHERE a.user_id = b.user_id
                      AND a.message_id = b.message_id
                      AND a.id > b.id
                `).Error; err != nil {
                    return err
                }
                // 2) Crea los índices únicos (idempotente con IF NOT EXISTS).
                if err := tx.Exec(`
                    CREATE UNIQUE INDEX IF NOT EXISTS idx_reaction_unique
                    ON message_reactions (message_id, user_id, emoji)
                `).Error; err != nil {
                    return err
                }
                return tx.Exec(`
                    CREATE UNIQUE INDEX IF NOT EXISTS idx_star_unique
                    ON starred_messages (user_id, message_id)
                `).Error
            },
            Rollback: func(tx *gorm.DB) error {
                if err := tx.Exec(`DROP INDEX IF EXISTS idx_reaction_unique`).Error; err != nil {
                    return err
                }
                return tx.Exec(`DROP INDEX IF EXISTS idx_star_unique`).Error
            },
        },
        {
            // Backfill (M-5): agrega a todos los usuarios activos de cada empresa
            // como miembros de los canales PÚBLICOS existentes de su empresa. Los
            // públicos se crean con todos los usuarios del momento, pero quienes
            // llegaron después no quedaban como miembros (no podían escribir bien,
            // ni tenían no-leídos/tiempo real). joined_at = NOW() para que NO vean
            // el historial viejo como no-leído (el conteo usa joined_at). Idempotente
            // (NOT EXISTS); la PK compuesta (channel_id, user_id) evita duplicados.
            ID: "202606181200_backfill_public_channel_members",
            Migrate: func(tx *gorm.DB) error {
                log.Println("Backfilling public channel members for existing company users...")
                return tx.Exec(`
                    INSERT INTO channel_members (channel_id, user_id, role, joined_at, created_at)
                    SELECT c.id, u.id, 'member', NOW(), NOW()
                    FROM channels c
                    JOIN users u ON (u.id = c.tenant_id OR u.empleador_id = c.tenant_id)
                    WHERE c.type = 'public'
                      AND c.is_active = true
                      AND c.deleted_at IS NULL
                      AND c.tenant_id > 0
                      AND u.is_active = true
                      AND u.deleted_at IS NULL
                      AND u.user_type NOT IN ('superadmin', 'customer_success')
                      AND NOT EXISTS (
                          SELECT 1 FROM channel_members cm
                          WHERE cm.channel_id = c.id AND cm.user_id = u.id
                      )
                `).Error
            },
            Rollback: func(tx *gorm.DB) error {
                return nil
            },
        },
        // Future migrations go here
    })

    if err := m.Migrate(); err != nil {
        log.Printf("Could not migrate: %v", err)
        return err
    }

    log.Println("Migration successful")
    return nil
}