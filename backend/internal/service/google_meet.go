package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// googleMeetAPIBase es la API de Meet, distinta de la de Calendar aunque
// comparta credencial: la sala la crea Calendar (conferenceData.createRequest) y
// quién está dentro solo lo sabe Meet.
const googleMeetAPIBase = "https://meet.googleapis.com/v2"

// ErrMeetScopeMissing: la credencial guardada se emitió antes de que la app
// pidiera meetings.space.readonly, así que Google devuelve 403. No es un fallo
// del sistema ni una revocación: el usuario lo resuelve reconectando, y hasta
// entonces todo lo demás (crear, editar, cancelar) sigue funcionando.
var ErrMeetScopeMissing = errors.New("vuelve a conectar tu cuenta de Google para ver quién está en la sala")

// MeetPresence describe quién hay ahora mismo en una sala.
type MeetPresence struct {
	// Live indica que hay una conferencia en curso. Sin ella el contador es 0 y
	// no significa "nadie se ha conectado nunca", sino "la sala está vacía ahora".
	Live bool `json:"live"`
	// Active son los participantes que siguen dentro.
	Active int `json:"active"`
	// Names son los nombres que Google reporta, cuando los reporta: los invitados
	// anónimos y los que entran por teléfono no siempre traen nombre.
	Names []string `json:"names,omitempty"`
}

// meetingCodePattern extrae el código de sala de una URL de Meet
// (https://meet.google.com/nnv-fbhe-wpc → nnv-fbhe-wpc). La API de Meet acepta
// ese código como alias del space, así que no hace falta guardar nada más.
var meetingCodePattern = regexp.MustCompile(`([a-z]{3}-[a-z]{4}-[a-z]{3})`)

// meetingCodeFromURL devuelve el código de sala, o vacío si la URL no lo lleva.
func meetingCodeFromURL(meetURL string) string {
	return meetingCodePattern.FindString(strings.ToLower(meetURL))
}

// --- Respuestas de la API de Meet ---

type meetSpaceResponse struct {
	Name string `json:"name"`
	// ActiveConference solo viene cuando hay gente dentro AHORA. Su ausencia es
	// la vía rápida para responder "sala vacía" sin listar participantes.
	ActiveConference *struct {
		ConferenceRecord string `json:"conferenceRecord"`
	} `json:"activeConference"`
}

type meetParticipantsResponse struct {
	Participants []struct {
		Name string `json:"name"`
		// LatestEndTime vacío = la persona sigue dentro. Es el criterio que la
		// propia API documenta para distinguir activos de históricos.
		LatestEndTime string `json:"latestEndTime"`
		SignedinUser  *struct {
			DisplayName string `json:"displayName"`
		} `json:"signedinUser"`
		AnonymousUser *struct {
			DisplayName string `json:"displayName"`
		} `json:"anonymousUser"`
		PhoneUser *struct {
			DisplayName string `json:"displayName"`
		} `json:"phoneUser"`
	} `json:"participants"`
	NextPageToken string `json:"nextPageToken"`
}

func (p meetParticipantsResponse) displayNames() []string {
	var names []string
	for _, participant := range p.Participants {
		switch {
		case participant.SignedinUser != nil && participant.SignedinUser.DisplayName != "":
			names = append(names, participant.SignedinUser.DisplayName)
		case participant.AnonymousUser != nil && participant.AnonymousUser.DisplayName != "":
			names = append(names, participant.AnonymousUser.DisplayName)
		case participant.PhoneUser != nil && participant.PhoneUser.DisplayName != "":
			names = append(names, participant.PhoneUser.DisplayName)
		}
	}
	return names
}

// MeetPresence consulta quién hay en la sala de un enlace de Meet, usando la
// credencial de userID (que debe ser dueño de la sala o participante).
//
// Son dos llamadas: primero el space —que dice si hay conferencia activa y cuál
// es su registro— y solo si la hay, los participantes filtrados por
// `latestEndTime IS NULL`, que es como la API marca a quien sigue dentro. Cuando
// la sala está vacía nos ahorramos la segunda.
func (s *googleCalendarService) MeetPresence(userID uint, meetURL string) (*MeetPresence, error) {
	if !s.enabled {
		return nil, ErrGoogleDisabled
	}
	code := meetingCodeFromURL(meetURL)
	if code == "" {
		return nil, fmt.Errorf("%w: el enlace no tiene un código de sala reconocible", ErrGooglePermanent)
	}

	space, err := s.meetSpace(userID, code)
	if err != nil {
		return nil, err
	}
	if space.ActiveConference == nil || space.ActiveConference.ConferenceRecord == "" {
		return &MeetPresence{Live: false}, nil
	}

	participants, err := s.meetActiveParticipants(userID, space.ActiveConference.ConferenceRecord)
	if err != nil {
		return nil, err
	}
	return &MeetPresence{
		Live:   true,
		Active: len(participants.Participants),
		Names:  participants.displayNames(),
	}, nil
}

func (s *googleCalendarService) meetSpace(userID uint, meetingCode string) (*meetSpaceResponse, error) {
	spaceURL := googleMeetAPIBase + "/spaces/" + url.PathEscape(meetingCode)
	resp, err := s.doGoogleRequest(userID, http.MethodGet, spaceURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, s.classifyMeetError(userID, resp)
	}
	var space meetSpaceResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&space); err != nil {
		return nil, err
	}
	return &space, nil
}

func (s *googleCalendarService) meetActiveParticipants(userID uint, conferenceRecord string) (*meetParticipantsResponse, error) {
	q := url.Values{}
	q.Set("filter", "latestEndTime IS NULL")
	// 100 es el tope de la API. Una reunión con más gente daría un contador corto,
	// pero para el aforo de una sesión de trabajo sobra y evita paginar.
	q.Set("pageSize", "100")

	participantsURL := googleMeetAPIBase + "/" + conferenceRecord + "/participants?" + q.Encode()
	resp, err := s.doGoogleRequest(userID, http.MethodGet, participantsURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, s.classifyMeetError(userID, resp)
	}
	var participants meetParticipantsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&participants); err != nil {
		return nil, err
	}
	return &participants, nil
}

// classifyMeetError distingue los tres finales que importan:
//
//   - 401 → credencial revocada (needs_reauth), como en Calendar.
//   - 403 → casi siempre el token es viejo y no lleva meetings.space.readonly.
//     Se separa de needs_reauth a propósito: la cuenta sigue sirviendo para todo
//     lo demás, y marcarla como revocada rompería crear y editar sesiones por un
//     contador de asistentes.
//   - 404 → la sala aún no existe (nadie ha entrado nunca). No es un error: es
//     una sala vacía, y así lo trata el que llama.
func (s *googleCalendarService) classifyMeetError(userID uint, resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		s.markNeedsReauth(userID, "Google rechazó la credencial al consultar la sala de Meet")
		return ErrNeedsReauth
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrMeetScopeMissing
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrEventGone
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if isTransientStatus(resp.StatusCode) {
		return fmt.Errorf("Google respondió %s al consultar la sala: %s", resp.Status, string(snippet))
	}
	return fmt.Errorf("%w (%s): %s", ErrGooglePermanent, resp.Status, string(snippet))
}
