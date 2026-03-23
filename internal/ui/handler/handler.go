package handler

import (
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	indigo "github.com/barn0w1/indigo-webarena-go"
	"github.com/barn0w1/indigo-webarena-go/internal/ui"
)

// Handler holds the SDK client and parsed templates for all UI routes.
type Handler struct {
	client *indigo.Client
	tmpls  map[string]*template.Template
	base   *template.Template
}

type pageData struct {
	Title   string
	Section string
	Flash   *flash
	Data    interface{}
}

type flash struct {
	Kind    string // "ok" | "err"
	Message string
}

// New parses all templates and returns a Handler ready to register routes.
func New(client *indigo.Client) *Handler {
	funcs := template.FuncMap{
		"itoa":            strconv.Itoa,
		"statusBadgeClass": statusBadgeClass,
	}

	base := template.New("").Funcs(funcs)
	base = template.Must(base.ParseFS(ui.TemplateFS,
		"templates/layout.html",
		"templates/partials/*.html",
	))

	pages := []string{
		"templates/instances.html",
		"templates/instance_create.html",
		"templates/instance_detail.html",
		"templates/sshkeys.html",
		"templates/sshkeys_new.html",
	}

	tmpls := make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		t := template.Must(base.Clone())
		t = template.Must(t.ParseFS(ui.TemplateFS, p))
		name := p[len("templates/"):]
		tmpls[name] = t
	}

	return &Handler{client: client, tmpls: tmpls, base: base}
}

// RegisterRoutes attaches all UI routes to mux using Go 1.22+ method+path patterns.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/instances", http.StatusFound)
	})

	mux.HandleFunc("GET /instances", h.handleInstanceList)
	mux.HandleFunc("GET /instances/new", h.handleInstanceNew)
	mux.HandleFunc("POST /instances", h.handleInstanceCreate)
	mux.HandleFunc("GET /instances/{id}", h.handleInstanceDetail)
	mux.HandleFunc("POST /instances/{id}/start", h.makeInstanceAction("start"))
	mux.HandleFunc("POST /instances/{id}/stop", h.makeInstanceAction("stop"))
	mux.HandleFunc("POST /instances/{id}/forcestop", h.makeInstanceAction("forcestop"))
	mux.HandleFunc("POST /instances/{id}/reset", h.makeInstanceAction("reset"))
	mux.HandleFunc("POST /instances/{id}/destroy", h.makeInstanceAction("destroy"))

	mux.HandleFunc("GET /sshkeys", h.handleSSHKeyList)
	mux.HandleFunc("GET /sshkeys/new", h.handleSSHKeyNew)
	mux.HandleFunc("POST /sshkeys", h.handleSSHKeyCreate)
	mux.HandleFunc("GET /sshkeys/{id}", h.handleSSHKeyView)
	mux.HandleFunc("GET /sshkeys/{id}/edit", h.handleSSHKeyEdit)
	mux.HandleFunc("POST /sshkeys/{id}", h.handleSSHKeyUpdate)
	mux.HandleFunc("POST /sshkeys/{id}/delete", h.handleSSHKeyDelete)
}

// render executes the named full-page template (layout + content).
func (h *Handler) render(w http.ResponseWriter, name string, data pageData) {
	t, ok := h.tmpls[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("render", "template", name, "err", err)
	}
}

// renderPartial executes a named partial template (no layout wrapper).
func (h *Handler) renderPartial(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.base.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("renderPartial", "template", name, "err", err)
	}
}

// errFlash writes an error flash partial — used as the hx-target="#flash" response.
func (h *Handler) errFlash(w http.ResponseWriter, msg string) {
	h.renderPartial(w, "flash", &flash{Kind: "err", Message: msg})
}

// okFlash writes a success flash partial.
func (h *Handler) okFlash(w http.ResponseWriter, msg string) {
	h.renderPartial(w, "flash", &flash{Kind: "ok", Message: msg})
}

// pathInt extracts an integer path value from the request.
func pathInt(r *http.Request, name string) (int, error) {
	return strconv.Atoi(r.PathValue(name))
}

func statusBadgeClass(status string) string {
	switch status {
	case "running":
		return "badge badge-running"
	case "stopped", "stop":
		return "badge badge-stopped"
	case "starting", "stopping":
		return "badge badge-transitioning"
	default:
		return "badge badge-stopped"
	}
}
