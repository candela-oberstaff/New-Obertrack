package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
}

// AnalyzeSentiment conecta con Ollama Cloud para obtener el sentimiento de un texto
func AnalyzeSentiment(text string) string {
	// 1. Obtener credenciales de las variables de entorno de Coolify
	apiURL := os.Getenv("OLLAMA_API_URL") // Ej: https://api.ollama.com/v1 o la de tu proveedor
	apiKey := os.Getenv("OLLAMA_API_KEY")
	model := os.Getenv("OLLAMA_MODEL")   // Ej: llama3, mistral, etc.

	if apiURL == "" {
		apiURL = "https://api.ollama.com" // URL base por defecto de respaldo
	}
	if model == "" {
		model = "llama3" // Modelo por defecto
	}

	// 2. Crear un prompt estricto para evitar respuestas largas o alucinaciones
	prompt := fmt.Sprintf(
		"Analiza el sentimiento del siguiente ticket de soporte de manera objetiva. "+
		"Responde ÚNICAMENTE con una de estas opciones en mayúsculas: POSITIVO, NEUTRAL, NEGATIVO o URGENTE. "+
		"No incluyas saludos, explicaciones, ni signos de puntuación. Texto: \"%s\"", 
		text,
	)

	requestBody, _ := json.Marshal(OllamaRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	})

	// 3. Configurar la petición HTTP
	req, err := http.NewRequest("POST", apiURL+"/api/generate", bytes.NewBuffer(requestBody))
	if err != nil {
		return "NEUTRAL"
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("[Ollama Error]: No se pudo conectar a Ollama Cloud ->", err)
		return "NEUTRAL" // Failsafe para no romper el flujo si la IA se cae
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[Ollama Error]: Estatus inesperado de la API: %d\n", resp.StatusCode)
		return "NEUTRAL"
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "NEUTRAL"
	}

	// Limpiar espacios en blanco o saltos de línea molestos de la respuesta
	result := strings.TrimSpace(ollamaResp.Response)
	return strings.ToUpper(result)
}