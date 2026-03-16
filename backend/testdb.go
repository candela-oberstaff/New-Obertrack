package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Try different SSL modes
	sslModes := []string{"require", "verify-ca", "verify-full", "prefer", "disable"}

	for _, mode := range sslModes {
		fmt.Printf("\n=== Testing sslmode=%s ===\n", mode)
		connStr := fmt.Sprintf("postgres://postgres:c47600098745015d7c01182250fe0923@y8uhrat7.us-west.database.insforge.app:5432/insforge?sslmode=%s", mode)

		conn, err := pgx.Connect(ctx, connStr)
		if err != nil {
			fmt.Printf("Failed: %v\n", err)
			continue
		}

		fmt.Println("Connected successfully!")

		var result int
		err = conn.QueryRow(ctx, "SELECT 1").Scan(&result)
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)
		} else {
			fmt.Println("Query result:", result)
		}
		conn.Close(ctx)
		break
	}
}
