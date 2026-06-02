package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// uptr is a tiny helper to take the address of a uint literal.
func uptr(v uint) *uint { return &v }

// --- models.TenantForUser ---------------------------------------------------

func TestTenantForUser(t *testing.T) {
	employer := &models.User{ID: 10, UserType: models.UserTypeEmployer}
	if got := models.TenantForUser(employer); got != 10 {
		t.Fatalf("employer tenant: want 10, got %d", got)
	}

	pro := &models.User{ID: 20, UserType: models.UserTypeProfessional, EmpleadorID: uptr(10)}
	if got := models.TenantForUser(pro); got != 10 {
		t.Fatalf("professional tenant: want 10, got %d", got)
	}

	orphan := &models.User{ID: 30, UserType: models.UserTypeProfessional}
	if got := models.TenantForUser(orphan); got != 0 {
		t.Fatalf("orphan tenant: want 0, got %d", got)
	}

	if got := models.TenantForUser(nil); got != 0 {
		t.Fatalf("nil tenant: want 0, got %d", got)
	}
}

// --- authorizeUserTenant (object-level access control) ----------------------

func TestAuthorizeUserTenant_CrossTenantDenied(t *testing.T) {
	s := &userService{}
	// Target belongs to tenant 99; requester is in tenant 1.
	target := &models.User{ID: 500, UserType: models.UserTypeProfessional, EmpleadorID: uptr(99)}

	err := s.authorizeUserTenant(target, /*requesterID*/ 1, /*tenantID*/ 1,
		/*isSuperadmin*/ false, /*requireManage*/ false, "", false)
	if err == nil {
		t.Fatal("expected cross-tenant access to be denied, got nil")
	}
}

func TestAuthorizeUserTenant_SameTenantAllowed(t *testing.T) {
	s := &userService{}
	target := &models.User{ID: 500, UserType: models.UserTypeProfessional, EmpleadorID: uptr(7)}

	// Employer of tenant 7 (tenantID 7) reading a member: allowed.
	if err := s.authorizeUserTenant(target, 7, 7, false, false, "empleador", false); err != nil {
		t.Fatalf("same-tenant read should be allowed, got %v", err)
	}
}

func TestAuthorizeUserTenant_SelfAllowed(t *testing.T) {
	s := &userService{}
	target := &models.User{ID: 42, UserType: models.UserTypeProfessional, EmpleadorID: uptr(7)}

	// A user can always act on themselves regardless of tenant context.
	if err := s.authorizeUserTenant(target, 42, 0, false, false, "profesional", false); err != nil {
		t.Fatalf("self access should be allowed, got %v", err)
	}
}

func TestAuthorizeUserTenant_SuperadminBypass(t *testing.T) {
	s := &userService{}
	target := &models.User{ID: 500, UserType: models.UserTypeProfessional, EmpleadorID: uptr(99)}

	if err := s.authorizeUserTenant(target, 1, 1, true, true, "superadmin", false); err != nil {
		t.Fatalf("superadmin should bypass tenant checks, got %v", err)
	}
}

func TestAuthorizeUserTenant_RequireManage_DeniesPlainProfessional(t *testing.T) {
	s := &userService{}
	// Same tenant (7) but the requester is a plain professional (not manager,
	// not employer) trying to manage another user. Must be denied (privilege
	// escalation guard).
	target := &models.User{ID: 501, UserType: models.UserTypeProfessional, EmpleadorID: uptr(7)}

	err := s.authorizeUserTenant(target, /*requesterID*/ 200, /*tenantID*/ 7,
		false, /*requireManage*/ true, "profesional", /*isManager*/ false)
	if err == nil {
		t.Fatal("plain professional must not manage other users, got nil")
	}
}

func TestAuthorizeUserTenant_RequireManage_AllowsManager(t *testing.T) {
	s := &userService{}
	target := &models.User{ID: 501, UserType: models.UserTypeProfessional, EmpleadorID: uptr(7)}

	if err := s.authorizeUserTenant(target, 200, 7, false, true, "profesional", true); err != nil {
		t.Fatalf("manager in same tenant should be allowed to manage, got %v", err)
	}
}

func TestAuthorizeUserTenant_ZeroTenantDenied(t *testing.T) {
	s := &userService{}
	target := &models.User{ID: 501, UserType: models.UserTypeProfessional, EmpleadorID: uptr(7)}

	// A requester with no tenant context (tenantID 0) must not reach other users.
	if err := s.authorizeUserTenant(target, 200, 0, false, false, "profesional", false); err == nil {
		t.Fatal("zero-tenant requester must be denied, got nil")
	}
}

// --- Password policy (audit M-08) -------------------------------------------

func TestValidatePasswordStrength(t *testing.T) {
	cases := []struct {
		pw    string
		valid bool
	}{
		{"short1", false},     // too short
		{"abcdefgh", false},   // no digit
		{"12345678", false},   // no letter
		{"abcd1234", true},    // ok
		{"Str0ngPass", true},  // ok
	}
	for _, c := range cases {
		err := ValidatePasswordStrength(c.pw)
		if c.valid && err != nil {
			t.Errorf("password %q should be valid, got %v", c.pw, err)
		}
		if !c.valid && err == nil {
			t.Errorf("password %q should be rejected", c.pw)
		}
	}
}
