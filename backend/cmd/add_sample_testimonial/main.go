package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/models"
)

func main() {
	cwd, _ := os.Getwd()
	paths := []string{
		filepath.Join(cwd, ".env"),
		filepath.Join(cwd, "..", ".env"),
		filepath.Join(cwd, "..", "..", ".env"),
		".env",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			godotenv.Load(p)
			log.Printf("Loaded env from: %s", p)
			break
		}
	}

	cfg := config.LoadConfig()
	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Buscar empresa Oberstaff (usuario tipo empleador o por company_name)
	var employer models.User
	if err := db.Where("user_type = ? AND (company_name ILIKE ? OR name ILIKE ?)", models.UserTypeEmployer, "%oberstaff%", "%oberstaff%").First(&employer).Error; err != nil {
		// Probar cualquier usuario con company_name oberstaff
		if err := db.Where("company_name ILIKE ? OR name ILIKE ?", "%oberstaff%", "%oberstaff%").First(&employer).Error; err != nil {
			log.Fatalf("No se encontró empresa con nombre Oberstaff: %v", err)
		}
	}
	fmt.Printf("Encontrada Empresa: ID=%d, Name=%s, CompanyName=%s\n", employer.ID, employer.Name, employer.CompanyName)

	// Buscar un profesional de esta empresa (por empleador_id o por employments)
	var professional models.User
	err = db.Where("user_type = ? AND empleador_id = ?", models.UserTypeProfessional, employer.ID).First(&professional).Error
	if err != nil {
		// Intentar buscar a través de la tabla employments
		var emp models.Employment
		if errEmp := db.Where("company_id = ? AND status = ?", employer.ID, "active").First(&emp).Error; errEmp == nil {
			db.First(&professional, emp.UserID)
		}
	}

	if professional.ID == 0 {
		// Si no tiene empleado asignado, buscar cualquier profesional activo para asociarlo temporalmente a esta empresa
		if errProf := db.Where("user_type = ?", models.UserTypeProfessional).First(&professional).Error; errProf != nil {
			log.Fatalf("No se encontró ningún profesional en la base de datos: %v", errProf)
		}
	}

	fmt.Printf("Encontrado Profesional: ID=%d, Name=%s, Email=%s\n", professional.ID, professional.Name, professional.Email)

	// Token aleatorio
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	now := time.Now()
	submittedAt := now.Add(-2 * time.Hour)
	signedAt := now.Add(-2 * time.Hour)
	expiresAt := now.Add(30 * 24 * time.Hour)

	type answerItem struct {
		Prompt string `json:"prompt"`
		Answer string `json:"answer"`
	}

	answers := []answerItem{
		{
			Prompt: "¿Cómo ha sido tu experiencia trabajando con el equipo de Oberstaff?",
			Answer: "Ha sido una experiencia muy enriquecedora y gratificante. El soporte técnico y la coordinación siempre están disponibles para resolver cualquier duda rápidamente.",
		},
		{
			Prompt: "¿Qué es lo que más valoras del acompañamiento y la cultura de trabajo?",
			Answer: "La claridad en los objetivos, la comunicación fluida y la confianza que brindan para desarrollar soluciones de alto impacto con autonomía.",
		},
		{
			Prompt: "¿Recomendarías formar parte del equipo de profesionales?",
			Answer: "Totalmente. Es un ambiente que impulsa el crecimiento continuo y el compromiso con la excelencia.",
		},
	}
	answersJSON, _ := json.Marshal(answers)

	prompts := []string{
		"¿Cómo ha sido tu experiencia trabajando con el equipo de Oberstaff?",
		"¿Qué es lo que más valoras del acompañamiento y la cultura de trabajo?",
		"¿Recomendarías formar parte del equipo de profesionales?",
	}
	promptsJSON, _ := json.Marshal(prompts)

	// Buscar un admin que figure como requestedBy
	var adminUser models.User
	if err := db.Where("is_superadmin = true OR user_type = ?", models.UserTypeSuperadmin).First(&adminUser).Error; err != nil {
		adminUser.ID = 1
	}

	companyTitle := employer.CompanyName
	if companyTitle == "" {
		companyTitle = employer.Name
	}

	testimonial := models.Testimonial{
		Token:            token,
		Audience:         "professional",
		Status:           "approved",
		UserID:           professional.ID,
		RequestedBy:      adminUser.ID,
		RecipientName:    professional.Name,
		RecipientEmail:   professional.Email,
		RecipientRole:    "Desarrollador / Profesional",
		RecipientCompany: companyTitle,
		Prompts:          string(promptsJSON),
		IntroMessage:     "Hola " + professional.Name + ", nos encantaría contar con tu testimonio sobre tu experiencia en " + companyTitle + ".",
		ConsentText:      "Autorizo expresamente a " + companyTitle + " a utilizar mi testimonio, nombre y cargo para fines institucionales y de difusión.",
		ConsentVersion:   "v1.0",
		ExpiresAt:        expiresAt,
		Rating:           5,
		Quote:            "Trabajar con Oberstaff ha sido una experiencia sumamente positiva. La comunicación y el respaldo técnico han sido impecables en todo momento.",
		PublishedQuote:   "Trabajar con Oberstaff ha sido una experiencia sumamente positiva. La comunicación y el respaldo técnico han sido impecables.",
		Answers:          string(answersJSON),
		SubmittedAt:      &submittedAt,
		AllowPublicName:  true,
		AllowRole:        true,
		AllowPhoto:       true,
		AllowLogo:        false,
		SignatureName:    professional.Name,
		SignatureMode:    "typed",
		SignedAt:         &signedAt,
		SignerIP:         "192.168.1.105",
		SignerUserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
		ReviewNote:       "Testimonio de prueba generado para verificar la visualización en expediente.",
	}

	if err := db.Create(&testimonial).Error; err != nil {
		log.Fatalf("Error al crear testimonio de prueba: %v", err)
	}

	fmt.Printf("\n>>> ¡Testimonio de prueba creado exitosamente!\nID: %d\nProfesional: %s (User ID: %d)\nEmpresa: %s (Tenant ID: %d)\nEstado: %s\nRating: %d estrellas\n",
		testimonial.ID, professional.Name, professional.ID, companyTitle, employer.ID, testimonial.Status, testimonial.Rating)
}
