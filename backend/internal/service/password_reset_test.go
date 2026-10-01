package service

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/clause"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// fakeResetUserRepo cubre lo que usan Login y ResetPassword; el resto de la
// interfaz queda sin implementar (entra en pánico si se llama).
type fakeResetUserRepo struct {
	repository.UserRepository
	byEmail     map[string]*models.User
	byID        map[uint]*models.User
	askedEmails []string
	updates     []map[string]interface{}
}

func (f *fakeResetUserRepo) GetByEmail(email string) (*models.User, error) {
	f.askedEmails = append(f.askedEmails, email)
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, errors.New("record not found")
}

func (f *fakeResetUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, errors.New("record not found")
}

func (f *fakeResetUserRepo) Update(_ *models.User, updates map[string]interface{}) error {
	f.updates = append(f.updates, updates)
	return nil
}

// Una temporal tiene que pasar la regla de contraseñas siempre: con 57
// caracteres y solo 8 dígitos, ~1 de cada 6 claves de 12 salía sin números.
func TestGenerateTempPasswordAlwaysPassesPolicy(t *testing.T) {
	for i := 0; i < 3000; i++ {
		pw, err := GenerateTempPassword(12)
		if err != nil {
			t.Fatal(err)
		}
		if len(pw) != 12 {
			t.Fatalf("longitud %d: %q", len(pw), pw)
		}
		if err := ValidatePasswordStrength(pw); err != nil {
			t.Fatalf("%q no cumple la regla: %v", pw, err)
		}
		for _, c := range pw {
			if !strings.ContainsRune(tempPasswordAlphabet, c) {
				t.Fatalf("%q tiene un carácter fuera del alfabeto: %q", pw, c)
			}
		}
	}
}

// El correo del login no distingue mayúsculas ni espacios alrededor: el
// móvil pone la primera letra en mayúscula y al copiar se cuela un espacio.
func TestLoginNormalizesEmail(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("QaTemp7xYz2k"), bcrypt.MinCost)
	user := &models.User{ID: 9, Email: "ana@x.com", Password: string(hash), UserType: models.UserTypeProfessional, IsActive: true}
	repo := &fakeResetUserRepo{byEmail: map[string]*models.User{"ana@x.com": user}}
	svc := NewAuthService(repo, "secret", nil)

	for _, typed := range []string{"ana@x.com", "Ana@x.com", "  ANA@X.COM ", "ana@x.com\t"} {
		if _, _, _, err := svc.Login(typed, "QaTemp7xYz2k"); err != nil {
			t.Errorf("login con %q: %v", typed, err)
		}
	}
	for _, asked := range repo.askedEmails {
		if asked != "ana@x.com" {
			t.Errorf("se buscó %q en vez del correo normalizado", asked)
		}
	}

	// La contraseña NO se normaliza: un espacio de más sigue siendo otra clave.
	if _, _, _, err := svc.Login("ana@x.com", "QaTemp7xYz2k "); err == nil {
		t.Error("una contraseña con un espacio de más no debe entrar")
	}
}

func TestAdminResetPasswordRejectsWeakPassword(t *testing.T) {
	repo := &fakeResetUserRepo{byID: map[uint]*models.User{9: {ID: 9}}}
	svc := &adminService{userRepo: repo}

	for _, weak := range []string{"123", "abcdefgh", "12345678", "short1"} {
		err := svc.ResetPassword(9, weak)
		var w *WeakPasswordError
		if !errors.As(err, &w) {
			t.Errorf("%q: esperaba WeakPasswordError, got %v", weak, err)
		}
	}
	if len(repo.updates) != 0 {
		t.Fatalf("una contraseña débil no debe tocar la cuenta: %v", repo.updates)
	}
}

// El reset del panel cierra las sesiones abiertas, igual que el reset por
// enlace: sube token_version en el mismo UPDATE que la contraseña.
func TestAdminResetPasswordRevokesSessions(t *testing.T) {
	repo := &fakeResetUserRepo{byID: map[uint]*models.User{9: {ID: 9}}}
	svc := &adminService{userRepo: repo}

	if err := svc.ResetPassword(9, "QaTemp7xYz2k"); err != nil {
		t.Fatal(err)
	}
	if len(repo.updates) != 1 {
		t.Fatalf("esperaba un solo UPDATE, got %d", len(repo.updates))
	}
	u := repo.updates[0]
	hash, _ := u["password"].(string)
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("QaTemp7xYz2k")) != nil {
		t.Error("el hash guardado no corresponde a la contraseña nueva")
	}
	expr, ok := u["token_version"].(clause.Expr)
	if !ok || expr.SQL != "token_version + 1" {
		t.Errorf("token_version debe subir en el mismo UPDATE, got %#v", u["token_version"])
	}
}
