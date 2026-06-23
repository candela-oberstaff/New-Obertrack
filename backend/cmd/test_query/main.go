package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Define Role model with gorm:"->;-:migration" exactly like origin/main
type RoleTest struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	TenantID    uint           `gorm:"not null;index" json:"tenant_id"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Permissions string         `gorm:"type:text;not null;default:'{}'" json:"permissions"`
	CreatedBy   uint           `gorm:"not null" json:"created_by"`
	UserCount   int64          `gorm:"-" json:"user_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (RoleTest) TableName() string {
	return "roles"
}

func main() {
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSL_MODE"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Running GetUserRoles query with RoleTest (gorm:\"->;-:migration\")...")
	
	// Create a dummy assignment so we have user_count > 0 for role ID 1
	_ = db.Exec("DELETE FROM user_roles WHERE user_id = 999").Error
	err = db.Exec("INSERT INTO user_roles (user_id, role_id, created_at) VALUES (?, ?, ?)", 999, 1, time.Now()).Error
	if err != nil {
		fmt.Printf("Failed to insert test user_role: %v\n", err)
	}

	var roles []RoleTest
	err = db.Model(&RoleTest{}).
		Joins("JOIN user_roles ur ON ur.role_id = roles.id").
		Where("ur.user_id = ?", 999).
		Order("LOWER(roles.name) ASC").
		Find(&roles).Error

	if err != nil {
		fmt.Printf("\n[PROVED BUG] Query failed: %v\n", err)
	} else {
		fmt.Printf("\nQuery succeeded: found %d roles\n", len(roles))
	}

	fmt.Println("Running ListRoles style query with custom struct scan to populate gorm:\"-\" field...")
	type roleWithCount struct {
		RoleTest
		ScannedCount int64 `gorm:"column:user_count"`
	}
	var customList []roleWithCount
	err = db.Model(&RoleTest{}).
		Select("roles.*, (SELECT COUNT(*) FROM user_roles ur WHERE ur.role_id = roles.id) as user_count").
		Scan(&customList).Error
	if err != nil {
		fmt.Printf("Custom list query failed: %v\n", err)
	} else {
		fmt.Printf("Custom list query succeeded: found %d roles\n", len(customList))
		for i := range customList {
			customList[i].RoleTest.UserCount = customList[i].ScannedCount
			fmt.Printf("Role ID: %d, Role Name: %s, UserCount mapped value: %d\n", customList[i].RoleTest.ID, customList[i].RoleTest.Name, customList[i].RoleTest.UserCount)
		}
	}

	fmt.Println("Checking actual counts in database user_roles table...")
	rows, err := db.Raw("SELECT role_id, COUNT(*) FROM user_roles GROUP BY role_id").Rows()
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var roleID uint
			var count int64
			rows.Scan(&roleID, &count)
			fmt.Printf("Database - RoleID: %d, Count: %d\n", roleID, count)
		}
	} else {
		fmt.Printf("DB raw count query failed: %v\n", err)
	}
}
