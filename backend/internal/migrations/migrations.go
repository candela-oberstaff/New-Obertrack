package migrations

import (
	"log"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

func Run(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
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
			Rollback: func(tx *gorm.DB) error {
				// No need for rollback for the initial migration in this context
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
