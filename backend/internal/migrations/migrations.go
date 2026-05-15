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
		// Future migrations go here
	})

	if err := m.Migrate(); err != nil {
		log.Printf("Could not migrate: %v", err)
		return err
	}

	log.Println("Migration successful")
	return nil
}
