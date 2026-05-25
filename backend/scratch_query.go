package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/models"
)

func main() {
	cwd, _ := os.Getwd()
	godotenv.Load(filepath.Join(cwd, ".env"))

	cfg := config.LoadConfig()
	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	var tickets []models.Ticket
	if err := db.Preload("Contact").Find(&tickets).Error; err != nil {
		log.Fatalf("Failed to fetch tickets: %v", err)
	}

	fmt.Printf("Fetched %d tickets:\n", len(tickets))
	for _, t := range tickets {
		b, _ := json.MarshalIndent(t, "", "  ")
		fmt.Println(string(b))
	}
}
