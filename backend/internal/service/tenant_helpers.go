package service

import "github.com/obertrack/backend/internal/models"

func tenantForUser(user *models.User) uint {
	return models.TenantForUser(user)
}

func isEmployerRole(role string) bool {
	return role == string(models.UserTypeEmployer) || role == "empleador"
}
