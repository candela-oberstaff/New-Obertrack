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
				return tx.AutoMigrate(
					&models.User{},
					&models.Board{},
					&models.BoardMember{},
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
		// Future migrations go here
	})

	if err := m.Migrate(); err != nil {
		log.Printf("Could not migrate: %v", err)
		return err
	}

	log.Println("Migration successful")
	return nil
}
