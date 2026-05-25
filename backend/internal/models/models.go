package models

import (
	"log"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&User{},
		&Board{},
		&BoardMember{},
		&Task{},
		&TaskUser{},
		&Comment{},
		&TaskAttachment{},
		&WorkHour{},
		&Notification{},
		&MassEmailLog{},
		&Message{},
		&Channel{},
		&ChannelMember{},
		&ChannelMessage{},
		&MessageReaction{},
		&StarredMessage{},
		&UserStatus{},
		&Mention{},
		&EmailTemplate{},
		&EmailCampaign{},
		&Survey{},
		&SurveyQuestion{},
		&SurveyResponse{},
		&SurveyAnswer{},
		&Contact{},
		&Ticket{},
		&TicketMessage{},
	)
	if err != nil {
		log.Printf("Migration warning: %v", err)
	}
	return nil
}
