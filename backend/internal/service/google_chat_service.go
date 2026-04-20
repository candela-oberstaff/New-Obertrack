package service

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/api/chat/v1"
	"google.golang.org/api/option"
)

type GoogleChatService interface {
	SendDirectMessage(email string, messageText string) error
	SendWorkHourCard(email string, whID uint, professionalName string, date string, hours float64, activities string) error
}

type googleChatService struct {
	client *chat.Service
	ctx    context.Context
}

func NewGoogleChatService() GoogleChatService {
	ctx := context.Background()

	credentialsFile := os.Getenv("GOOGLE_CREDENTIALS_PATH")
	if credentialsFile == "" {
		credentialsFile = "credentials.json" // Default fallback
	}

	// Make sure the file exists; if not, return a dummy service that just logs
	// This prevents the whole backend from failing to start if credentials aren't set up yet
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		log.Printf("[Google Chat] Warning: credentials file '%s' not found. Chat integration will be disabled.", credentialsFile)
		return &googleChatService{client: nil, ctx: ctx}
	}

	// Create the Chat service client
	// Requires chat.bot scope for bot authentication
	client, err := chat.NewService(ctx, option.WithCredentialsFile(credentialsFile), option.WithScopes("https://www.googleapis.com/auth/chat.bot"))
	if err != nil {
		log.Printf("[Google Chat] Error initializing Google Chat Client: %v", err)
		return &googleChatService{client: nil, ctx: ctx}
	}

	log.Println("[Google Chat] Integration successfully initialized")
	return &googleChatService{
		client: client,
		ctx:    ctx,
	}
}

func (s *googleChatService) SendDirectMessage(email string, messageText string) error {
	// If client is nil, integration is disabled
	if s.client == nil {
		log.Printf("[Google Chat Log] Would send DM to %s: %s", email, messageText)
		return nil
	}

	// 1. Find the Direct Message Space with the user
	// The target user name format is users/{email}
	targetUser := fmt.Sprintf("users/%s", email)
	
	log.Printf("[Google Chat] Attempting to find DM space for: %s", targetUser)
	space, err := s.client.Spaces.FindDirectMessage().Name(targetUser).Context(s.ctx).Do()
	if err != nil {
		log.Printf("[Google Chat] Error finding space for %s: %v. (Note: The user might need to add the bot first)", email, err)
		return err
	}

	log.Printf("[Google Chat] Found space: %s for user: %s", space.Name, email)

	// 2. Send the message
	msg := &chat.Message{
		Text: messageText,
	}

	if _, err := s.client.Spaces.Messages.Create(space.Name, msg).Do(); err != nil {
		log.Printf("[Google Chat] Error sending message to %s (space: %s): %v", email, space.Name, err)
		return err
	}

	log.Printf("[Google Chat] DM successfully sent to %s", email)
	return nil
}
func (s *googleChatService) SendWorkHourCard(email string, whID uint, professionalName string, date string, hours float64, activities string) error {
	if s.client == nil {
		log.Printf("[Google Chat Log] Would send Card to %s for WH %d", email, whID)
		return nil
	}

	targetUser := fmt.Sprintf("users/%s", email)
	log.Printf("[Google Chat] Attempting to find DM space for card (WH %d): %s", whID, targetUser)
	space, err := s.client.Spaces.FindDirectMessage().Name(targetUser).Context(s.ctx).Do()
	if err != nil {
		log.Printf("[Google Chat] Error finding space for card to %s: %v", email, err)
		return err
	}

	// Build the Card V2
	card := &chat.CardWithId{
		CardId: "workhour_approval_card",
		Card: &chat.GoogleAppsCardV1Card{
			Header: &chat.GoogleAppsCardV1CardHeader{
				Title:    "Nueva Jornada Pendiente",
				Subtitle: fmt.Sprintf("Reportado por %s", professionalName),
				ImageUrl: "https://fonts.gstatic.com/s/i/short-term/release/googlesymbols/timer/default/48px.svg",
			},
			Sections: []*chat.GoogleAppsCardV1Section{
				{
					Widgets: []*chat.GoogleAppsCardV1Widget{
						{
							DecoratedText: &chat.GoogleAppsCardV1DecoratedText{
								TopLabel: "Detalles de la Jornada",
								Text:     fmt.Sprintf("📅 *Fecha:* %s\n⏱️ *Horas:* %.2f", date, hours),
							},
						},
						{
							DecoratedText: &chat.GoogleAppsCardV1DecoratedText{
								TopLabel: "Actividades",
								Text:     activities,
								WrapText: true,
							},
						},
						{
							ButtonList: &chat.GoogleAppsCardV1ButtonList{
								Buttons: []*chat.GoogleAppsCardV1Button{
									{
										Text: "Aprobar Jornada",
										OnClick: &chat.GoogleAppsCardV1OnClick{
											Action: &chat.GoogleAppsCardV1Action{
												Function: "approve_workhour",
												Parameters: []*chat.GoogleAppsCardV1ActionParameter{
													{
														Key:   "workhour_id",
														Value: fmt.Sprintf("%d", whID),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	msg := &chat.Message{
		CardsV2: []*chat.CardWithId{card},
	}

	if _, err := s.client.Spaces.Messages.Create(space.Name, msg).Do(); err != nil {
		log.Printf("[Google Chat] Error sending card to %s: %v", email, err)
		return err
	}

	log.Printf("[Google Chat] Card successfully sent to %s for WorkHour %d", email, whID)
	return nil
}
