package service

import (
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
)

func TestNormalizeAudience(t *testing.T) {
	cases := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"", models.TutorialAudienceAll, false},
		{"  ", models.TutorialAudienceAll, false},
		{"all", models.TutorialAudienceAll, false},
		{"empleador", models.TutorialAudienceEmployer, false},
		{"profesional", models.TutorialAudienceProfessional, false},
		{"superadmin", "", true},
		{"empresa", "", true},
	}
	for _, tc := range cases {
		got, err := normalizeAudience(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("normalizeAudience(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("normalizeAudience(%q) = %q, esperaba %q", tc.input, got, tc.want)
		}
	}
}

func TestValidateVideoURL(t *testing.T) {
	valid := []string{
		"https://drive.google.com/file/d/abc123XYZ/view",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ",
	}
	for _, url := range valid {
		if err := validateVideoURL(url); err != nil {
			t.Errorf("URL válida rechazada %q: %v", url, err)
		}
	}

	invalid := []string{
		"",
		"https://vimeo.com/12345",
		"https://drive.google.com/drive/folders/abc",
	}
	for _, url := range invalid {
		if err := validateVideoURL(url); err == nil {
			t.Errorf("URL inválida aceptada: %q", url)
		}
	}
}

func TestAnnounceRecipientTypes(t *testing.T) {
	has := func(types []models.UserType, want models.UserType) bool {
		for _, ut := range types {
			if ut == want {
				return true
			}
		}
		return false
	}

	// El superadmin recibe siempre: es quien ve la página sin filtro de
	// audiencia (a quien publica se le excluye después, por ID).
	for _, audience := range []string{models.TutorialAudienceAll, models.TutorialAudienceEmployer, models.TutorialAudienceProfessional} {
		if !has(announceRecipientTypes(audience), models.UserTypeSuperadmin) {
			t.Errorf("audiencia %q: el superadmin debería recibir el anuncio", audience)
		}
	}

	all := announceRecipientTypes(models.TutorialAudienceAll)
	if !has(all, models.UserTypeEmployer) || !has(all, models.UserTypeProfessional) {
		t.Error("audiencia 'all' debería alcanzar a empresas y profesionales")
	}

	employer := announceRecipientTypes(models.TutorialAudienceEmployer)
	if !has(employer, models.UserTypeEmployer) || has(employer, models.UserTypeProfessional) {
		t.Error("audiencia 'empleador' no debería alcanzar a los profesionales")
	}

	professional := announceRecipientTypes(models.TutorialAudienceProfessional)
	if !has(professional, models.UserTypeProfessional) || has(professional, models.UserTypeEmployer) {
		t.Error("audiencia 'profesional' no debería alcanzar a las empresas")
	}

	// Ni IT ni customer_success ven el módulo: tampoco reciben el aviso.
	for _, audience := range []string{models.TutorialAudienceAll, models.TutorialAudienceEmployer, models.TutorialAudienceProfessional} {
		types := announceRecipientTypes(audience)
		if has(types, models.UserTypeITAnalyst) || has(types, models.UserTypeCustomerSuccess) {
			t.Errorf("audiencia %q: IT y customer_success no deberían recibir novedades", audience)
		}
	}
}

func TestAnnouncementSummary(t *testing.T) {
	if got := announcementSummary("   "); got == "" {
		t.Error("una novedad sin descripción debe traer igual un cuerpo para la campanita")
	}
	if got := announcementSummary("Cambios en el registro de horas"); got != "Cambios en el registro de horas" {
		t.Errorf("descripción corta alterada: %q", got)
	}

	long := strings.Repeat("a", 400)
	got := announcementSummary(long)
	if len([]rune(got)) > 181 {
		t.Errorf("descripción larga sin recortar: %d runas", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("el recorte debería marcarse con puntos suspensivos: %q", got)
	}
}

func TestNormalizeAnnounceDays(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		// El 0 es una elección válida: publicar sin interrumpir a nadie.
		{0, 0},
		{1, 1},
		{2, 2},
		{30, 30},
		{maxAnnounceDays, maxAnnounceDays},
		// Fuera de rango: el negativo es "no me lo mandaron" y el excesivo se
		// recorta en lugar de rechazar la publicación.
		{-1, defaultAnnounceDays},
		{maxAnnounceDays + 1, maxAnnounceDays},
		{100000, maxAnnounceDays},
	}
	for _, tc := range cases {
		if got := normalizeAnnounceDays(tc.in); got != tc.want {
			t.Errorf("normalizeAnnounceDays(%d) = %d, esperaba %d", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeTutorialContentType(t *testing.T) {
	// Vacío = 'video': es lo único que existía antes de los otros formatos.
	for _, in := range []string{"", "   "} {
		if got, err := normalizeTutorialContentType(in); err != nil || got != models.TutorialContentVideo {
			t.Errorf("normalizeTutorialContentType(%q) = %q, %v; esperaba video sin error", in, got, err)
		}
	}
	for _, in := range []string{models.TutorialContentVideo, models.TutorialContentImage, models.TutorialContentText} {
		if got, err := normalizeTutorialContentType(in); err != nil || got != in {
			t.Errorf("normalizeTutorialContentType(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"pdf", "audio", "plantilla"} {
		if _, err := normalizeTutorialContentType(in); err == nil {
			t.Errorf("tipo inválido aceptado: %q", in)
		}
	}
}

func TestHasVisibleText(t *testing.T) {
	empty := []string{"", "   ", "<p></p>", "<p><br></p>", "<div>&nbsp;</div>", "<p><strong></strong></p>"}
	for _, html := range empty {
		if hasVisibleText(html) {
			t.Errorf("HTML sin texto tomado como contenido: %q", html)
		}
	}
	filled := []string{"Hola", "<p>Hola equipo</p>", "<ul><li>Uno</li></ul>"}
	for _, html := range filled {
		if !hasVisibleText(html) {
			t.Errorf("HTML con texto tomado como vacío: %q", html)
		}
	}
}

func TestValidateImageURL(t *testing.T) {
	valid := []string{"/api/uploads/1_2_flyer.png", "https://cdn.obertrack.com/a.png", "http://localhost/a.jpg"}
	for _, url := range valid {
		if err := validateImageURL(url); err != nil {
			t.Errorf("imagen válida rechazada %q: %v", url, err)
		}
	}
	// El data: y el javascript: en un <img> del anuncio irían a toda la empresa.
	invalid := []string{"", "   ", "javascript:alert(1)", "data:image/png;base64,AAAA", "ftp://x/a.png"}
	for _, url := range invalid {
		if err := validateImageURL(url); err == nil {
			t.Errorf("imagen inválida aceptada: %q", url)
		}
	}
}

func TestValidateContent(t *testing.T) {
	// Cada tipo exige SU campo y no le importan los otros.
	if err := validateContent(models.TutorialContentVideo, "https://youtu.be/dQw4w9WgXcQ", "", ""); err != nil {
		t.Errorf("video válido rechazado: %v", err)
	}
	if err := validateContent(models.TutorialContentVideo, "", "/api/uploads/a.png", "<p>hola</p>"); err == nil {
		t.Error("un video sin link no debería pasar aunque traiga imagen y texto")
	}
	if err := validateContent(models.TutorialContentImage, "", "/api/uploads/a.png", ""); err != nil {
		t.Errorf("imagen válida rechazada: %v", err)
	}
	if err := validateContent(models.TutorialContentImage, "https://youtu.be/dQw4w9WgXcQ", "", ""); err == nil {
		t.Error("una imagen sin archivo no debería pasar")
	}
	if err := validateContent(models.TutorialContentText, "", "", "<p>Cambios en el registro</p>"); err != nil {
		t.Errorf("texto válido rechazado: %v", err)
	}
	if err := validateContent(models.TutorialContentText, "", "", "<p><br></p>"); err == nil {
		t.Error("un texto vacío no debería pasar")
	}
}
