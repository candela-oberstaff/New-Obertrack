package service

import (
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
