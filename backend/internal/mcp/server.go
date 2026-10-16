package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/obertrack/backend/internal/models"
)

// Runtime es el contexto de UNA petición al endpoint MCP: quién llama, con
// qué alcance y con qué cuota. El servidor es sin estado: se construye un
// McpServer por petición con los tools que ese rol puede usar.
type Runtime struct {
	Svc         *Service
	RO          *ReadOnlyDB
	User        *models.User
	Conn        *models.MCPConnection
	Scope       *Scope
	Quota       models.MCPRoleQuota
	Settings    models.MCPSettings
	FrontendURL string
	Audit       AuditCtx

	toolNames []string
}

// Version del servidor que ven los clientes.
const ServerVersion = "1.0.0"

// NewServer construye el servidor MCP con los tools del rol.
func NewServer(rt *Runtime) *gomcp.Server {
	srv := gomcp.NewServer(&gomcp.Implementation{Name: "obertrack", Title: "Obertrack", Version: ServerVersion},
		&gomcp.ServerOptions{Instructions: ServerInstructions})
	registerTools(srv, rt)
	return srv
}

// ToolNames lista los tools registrados para este rol (whoami los muestra).
func (rt *Runtime) ToolNames() []string { return rt.toolNames }

// result es lo que devuelve la lógica de un tool.
type result struct {
	// Data se redacta (teléfonos, modo mínimo) y se serializa a JSON.
	Data any
	// Raw se serializa tal cual: para search/fetch, que tienen formato fijo y
	// ya redactan su contenido por dentro.
	Raw       any
	Rows      int
	Untrusted bool
}

// maxRows es el tope de filas de este rol; requested lo puede bajar.
func (rt *Runtime) maxRows(requested int) int {
	max := rt.Quota.MaxRows
	if max <= 0 {
		max = 50
	}
	if requested > 0 && requested < max {
		return requested
	}
	return max
}

func readOnlyAnnotations(title string) *gomcp.ToolAnnotations {
	closed := false
	return &gomcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed, Title: title}
}

// registerReadTool registra un tool de lectura con todo lo común: pausa,
// cuota, medición de tiempo, registro de uso, errores sin filtrar SQL ni
// stack traces, y la nota de texto no confiable.
func registerReadTool[In any](srv *gomcp.Server, rt *Runtime, name, title, desc string, quotaExempt bool,
	fn func(ctx context.Context, in In) (*result, error)) {
	rt.toolNames = append(rt.toolNames, name)
	gomcp.AddTool(srv, &gomcp.Tool{Name: name, Title: title, Description: desc, Annotations: readOnlyAnnotations(title)},
		func(ctx context.Context, _ *gomcp.CallToolRequest, in In) (*gomcp.CallToolResult, any, error) {
			if rt.Settings.Paused {
				return errorResult("El acceso de los asistentes de IA a Obertrack está pausado por un administrador. Vuelve a intentarlo más tarde."), nil, nil
			}
			var used int64
			if !quotaExempt {
				var blocked string
				used, blocked = rt.Svc.QuotaCheck(rt.User, rt.Quota, rt.Audit)
				if blocked != "" {
					return errorResult(blocked), nil, nil
				}
			}
			start := time.Now()
			res, err := safeRun(ctx, in, fn)
			ms := int(time.Since(start).Milliseconds())
			rows := 0
			if res != nil {
				rows = res.Rows
			}
			rt.Svc.RecordUsage(rt.User.ID, rt.Conn.ID, name, err == nil, ms, rows)
			if !quotaExempt {
				rt.Svc.AfterCall(rt.User, rt.Quota, used)
			}
			if err != nil {
				var ue *UserError
				if errors.As(err, &ue) {
					return errorResult(ue.Msg), nil, nil
				}
				log.Printf("[MCP] tool %s (usuario %d) falló: %v", name, rt.User.ID, err)
				return errorResult("No se pudo completar la consulta. Prueba con filtros más acotados o inténtalo de nuevo."), nil, nil
			}
			return rt.render(res), nil, nil
		})
}

// safeRun convierte un panic en error para que nunca llegue un stack trace
// al asistente.
func safeRun[In any](ctx context.Context, in In, fn func(context.Context, In) (*result, error)) (res *result, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[MCP] panic en tool: %v\n%s", r, debug.Stack())
			res, err = nil, fmt.Errorf("panic: %v", r)
		}
	}()
	return fn(ctx, in)
}

func errorResult(msg string) *gomcp.CallToolResult {
	return &gomcp.CallToolResult{IsError: true, Content: []gomcp.Content{&gomcp.TextContent{Text: msg}}}
}

func (rt *Runtime) render(res *result) *gomcp.CallToolResult {
	var payload any
	if res.Raw != nil {
		payload = res.Raw
	} else {
		payload = RedactData(normalize(res.Data), rt.Settings.MinimalData)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return errorResult("No se pudo preparar la respuesta.")
	}
	content := []gomcp.Content{&gomcp.TextContent{Text: string(b)}}
	if res.Untrusted {
		content = append(content, &gomcp.TextContent{Text: UntrustedTextNote})
	}
	return &gomcp.CallToolResult{Content: content}
}

// normalize pasa structs y mapas tipados a map[string]any / []any para que
// RedactData los pueda recorrer.
func normalize(v any) any {
	switch v.(type) {
	case map[string]any, []map[string]any, []any, string, nil:
		return v
	}
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if json.Unmarshal(b, &out) != nil {
		return v
	}
	return out
}
