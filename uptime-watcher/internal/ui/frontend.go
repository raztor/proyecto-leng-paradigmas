package ui

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"proyecto-leng-paradigmas/ejemplo/internal/model"
	"proyecto-leng-paradigmas/ejemplo/internal/service"
	"proyecto-leng-paradigmas/ejemplo/internal/store"
)

// Frontend renderiza una vista HTML simple con el estado de los servicios y expone formularios CRUD.
type Frontend struct {
	store *store.Store
	svc   *service.TargetService
	tpl   *template.Template
}

// New crea una instancia lista para usar.
func New(store *store.Store, svc *service.TargetService) (*Frontend, error) {
	funcs := template.FuncMap{
		"since": func(t *model.CheckResult) string {
			if t == nil {
				return "-"
			}
			return time.Since(t.CheckedAt).Round(time.Second).String()
		},
		"latency": func(t *model.CheckResult) string {
			if t == nil || t.Duration <= 0 {
				return "-"
			}
			return t.Duration.Round(time.Millisecond).String()
		},
		"statusClass": func(status model.TargetStatus) string {
			if status.LastCheck == nil {
				return "unknown"
			}
			if status.LastCheck.Success {
				return "up"
			}
			return "down"
		},
		"formatDuration": func(d time.Duration) string {
			if d <= 0 {
				return ""
			}
			return d.String()
		},
		"isHTTP": func(kind model.TargetKind) bool {
			return kind == model.TargetHTTP
		},
		"isTCP": func(kind model.TargetKind) bool {
			return kind == model.TargetTCP
		},
		"portAsString": func(port int) string {
			if port == 0 {
				return ""
			}
			return strconv.Itoa(port)
		},
	}
	tpl, err := template.New("index").Funcs(funcs).Parse(indexTemplate)
	if err != nil {
		return nil, err
	}
	return &Frontend{
		store: store,
		svc:   svc,
		tpl:   tpl,
	}, nil
}

// ServeHTTP implementa http.Handler para servir la vista principal.
func (f *Frontend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	query := r.URL.Query()
	data := struct {
		GeneratedAt time.Time
		Statuses    []model.TargetStatus
		Flash       struct {
			Success string
			Error   string
		}
	}{
		GeneratedAt: time.Now(),
		Statuses:    f.store.Status(),
	}
	data.Flash.Success = query.Get("success")
	data.Flash.Error = query.Get("error")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = f.tpl.Execute(w, data)
}

// HandleCreate procesa el formulario de creacion desde la UI.
func (f *Frontend) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target, err := parseTargetForm(r, "")
	if err != nil {
		redirectWithFlash(w, r, "", err.Error())
		return
	}
	if _, err := f.svc.CreateTarget(r.Context(), target); err != nil {
		redirectWithFlash(w, r, "", err.Error())
		return
	}
	redirectWithFlash(w, r, "Servicio creado correctamente", "")
}

// HandleUpdate procesa el formulario de edicion.
func (f *Frontend) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	targetID := strings.TrimSpace(r.FormValue("id"))
	target, err := parseTargetForm(r, targetID)
	if err != nil {
		redirectWithFlash(w, r, "", err.Error())
		return
	}
	if _, err := f.svc.UpdateTarget(r.Context(), target); err != nil {
		redirectWithFlash(w, r, "", err.Error())
		return
	}
	redirectWithFlash(w, r, "Servicio actualizado", "")
}

// HandleDelete elimina un servicio desde la UI.
func (f *Frontend) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	if id == "" {
		redirectWithFlash(w, r, "", "id requerido para eliminar")
		return
	}
	if err := f.svc.DeleteTarget(r.Context(), id); err != nil {
		redirectWithFlash(w, r, "", err.Error())
		return
	}
	redirectWithFlash(w, r, "Servicio eliminado", "")
}

func parseTargetForm(r *http.Request, forcedID string) (model.Target, error) {
	if err := r.ParseForm(); err != nil {
		return model.Target{}, err
	}
	form := r.Form
	id := strings.TrimSpace(formValue(form, "id"))
	if forcedID != "" {
		id = forcedID
	}
	name := strings.TrimSpace(formValue(form, "name"))
	kind := model.TargetKind(strings.ToLower(strings.TrimSpace(formValue(form, "kind"))))
	urlValue := strings.TrimSpace(formValue(form, "url"))
	host := strings.TrimSpace(formValue(form, "host"))
	portStr := strings.TrimSpace(formValue(form, "port"))
	freqStr := strings.TrimSpace(formValue(form, "frequency"))
	timeoutStr := strings.TrimSpace(formValue(form, "timeout"))

	if freqStr == "" {
		freqStr = "30s"
	}
	if timeoutStr == "" {
		timeoutStr = "5s"
	}

	freq, timeout, err := service.ParseDurations(freqStr, timeoutStr)
	if err != nil {
		return model.Target{}, err
	}

	var port int
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return model.Target{}, err
		}
		port = p
	}

	target := model.Target{
		ID:        id,
		Name:      name,
		Kind:      kind,
		URL:       urlValue,
		Host:      host,
		Port:      port,
		Frequency: freq,
		Timeout:   timeout,
	}
	return target, nil
}

func formValue(form url.Values, key string) string {
	return form.Get(key)
}

func redirectWithFlash(w http.ResponseWriter, r *http.Request, success, errMsg string) {
	values := url.Values{}
	if success != "" {
		values.Set("success", success)
	}
	if errMsg != "" {
		values.Set("error", errMsg)
	}
	target := "/"
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

const indexTemplate = `
<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <title>Monitor de Servicios</title>
  <style>
	body { font-family: Helvetica, Arial, sans-serif; background: #0f172a; color: #e2e8f0; margin: 0; padding: 0; }
	header { padding: 1.5rem; background: #1e293b; box-shadow: 0 2px 6px rgba(0,0,0,0.3); }
	h1 { margin: 0; font-size: 1.6rem; }
	main { padding: 1.5rem; display: grid; gap: 1.5rem; }
	table { width: 100%; border-collapse: collapse; background: #1e293b; border-radius: 12px; overflow: hidden; }
	th, td { padding: 0.75rem 1rem; text-align: left; vertical-align: top; }
	th { background: #0f172a; font-weight: 600; }
	tr:nth-child(even) { background: rgba(255,255,255,0.03); }
	.status-badge { padding: 0.25rem 0.6rem; border-radius: 999px; font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.08em; }
	.status-badge.up { background: rgba(34,197,94,0.2); color: #22c55e; }
	.status-badge.down { background: rgba(239,68,68,0.2); color: #ef4444; }
	.status-badge.unknown { background: rgba(148,163,184,0.2); color: #cbd5f5; }
	.footer { color: #94a3b8; font-size: 0.85rem; }
	a { color: #38bdf8; }
	.card { background: #1e293b; border-radius: 12px; padding: 1.25rem; box-shadow: 0 10px 30px rgba(15,23,42,0.4); }
	.card h2 { margin-top: 0; font-size: 1.2rem; }
	.form-grid { display: grid; gap: 0.75rem; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); }
	.form-grid label { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.85rem; color: #cbd5f5; }
	input, select { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 0.5rem 0.65rem; color: #e2e8f0; }
	input:focus, select:focus { outline: none; border-color: #38bdf8; box-shadow: 0 0 0 2px rgba(56,189,248,0.2); }
	button { padding: 0.55rem 1rem; border-radius: 999px; border: none; cursor: pointer; font-weight: 600; }
	.button-primary { background: linear-gradient(135deg, #38bdf8, #0ea5e9); color: #0f172a; }
	.button-danger { background: rgba(239,68,68,0.2); color: #ef4444; border: 1px solid rgba(239,68,68,0.4); }
	.actions { display: flex; gap: 0.5rem; flex-wrap: wrap; }
	.flash { padding: 0.75rem 1rem; border-radius: 10px; font-size: 0.95rem; }
	.flash.success { background: rgba(34,197,94,0.18); color: #4ade80; border: 1px solid rgba(34,197,94,0.3); }
	.flash.error { background: rgba(239,68,68,0.18); color: #f87171; border: 1px solid rgba(239,68,68,0.3); }
	details summary { cursor: pointer; color: #38bdf8; }
  </style>
</head>
<body>
  <header>
	<h1>Monitor de Servicios</h1>
	<p>Actualizado: {{ .GeneratedAt.Format "2006-01-02 15:04:05" }}</p>
  </header>
  <main>
	{{ if .Flash.Success }}<div class="flash success">{{ .Flash.Success }}</div>{{ end }}
	{{ if .Flash.Error }}<div class="flash error">{{ .Flash.Error }}</div>{{ end }}

	<section class="card">
	  <h2>Agregar nuevo servicio</h2>
	  <form class="form-grid" action="/ui/targets/create" method="post">
		<label>ID (opcional)
		  <input name="id" placeholder="uuid o slug" autocomplete="off">
		</label>
		<label>Nombre
		  <input name="name" required placeholder="Nombre descriptivo">
		</label>
		<label>Tipo
		  <select name="kind">
			<option value="http">HTTP</option>
			<option value="tcp">TCP</option>
		  </select>
		</label>
		<label>URL (HTTP)
		  <input name="url" placeholder="https://example.com/healthz">
		</label>
		<label>Host (TCP)
		  <input name="host" placeholder="localhost">
		</label>
		<label>Puerto (TCP)
		  <input name="port" type="number" min="1" max="65535" placeholder="5432">
		</label>
		<label>Frecuencia
		  <input name="frequency" value="30s" placeholder="ej: 30s, 1m">
		</label>
		<label>Timeout
		  <input name="timeout" value="5s" placeholder="ej: 5s">
		</label>
		<div class="actions">
		  <button type="submit" class="button-primary">Crear servicio</button>
		</div>
	  </form>
	</section>

	<section class="card">
	  <h2>Estado de los servicios</h2>
	  <table>
		<thead>
		  <tr>
			<th>Servicio</th>
			<th>Estado</th>
			<th>Último chequeo</th>
			<th>Latencia</th>
			<th>Uptime %</th>
			<th>Frecuencia</th>
			<th>Timeout</th>
			<th>Acciones</th>
		  </tr>
		</thead>
		<tbody>
		  {{- range .Statuses }}
		  <tr>
			<td>
			  <strong>{{ .Target.Name }}</strong><br>
			  <small>{{ .Target.Kind }} • {{ if eq .Target.Kind "http" }}{{ .Target.URL }}{{ else }}{{ .Target.Host }}:{{ .Target.Port }}{{ end }}</small>
			</td>
			<td><span class="status-badge {{ statusClass . }}">{{ if .LastCheck }}{{ if .LastCheck.Success }}UP{{ else }}DOWN{{ end }}{{ else }}Sin datos{{ end }}</span></td>
			<td>{{ since .LastCheck }}</td>
			<td>{{ latency .LastCheck }}</td>
			<td>{{ printf "%.1f" .UptimePerc }}</td>
			<td>{{ formatDuration .Target.Frequency }}</td>
			<td>{{ formatDuration .Target.Timeout }}</td>
			<td>
			  <details>
				<summary>Editar</summary>
				<form class="form-grid" action="/ui/targets/update" method="post" style="margin-top: 0.75rem;">
				  <input type="hidden" name="id" value="{{ .Target.ID }}">
				  <label>Nombre
					<input name="name" required value="{{ .Target.Name }}">
				  </label>
				  <label>Tipo
					<select name="kind">
					  <option value="http" {{ if isHTTP .Target.Kind }}selected{{ end }}>HTTP</option>
					  <option value="tcp" {{ if isTCP .Target.Kind }}selected{{ end }}>TCP</option>
					</select>
				  </label>
				  <label>URL (HTTP)
					<input name="url" value="{{ .Target.URL }}">
				  </label>
				  <label>Host (TCP)
					<input name="host" value="{{ .Target.Host }}">
				  </label>
				  <label>Puerto (TCP)
					<input name="port" type="number" min="1" max="65535" value="{{ portAsString .Target.Port }}">
				  </label>
				  <label>Frecuencia
					<input name="frequency" value="{{ formatDuration .Target.Frequency }}">
				  </label>
				  <label>Timeout
					<input name="timeout" value="{{ formatDuration .Target.Timeout }}">
				  </label>
				  <div class="actions">
					<button type="submit" class="button-primary">Guardar</button>
				  </div>
				</form>
				<form action="/ui/targets/delete" method="post" style="margin-top: 0.5rem;">
				  <input type="hidden" name="id" value="{{ .Target.ID }}">
				  <button type="submit" class="button-danger" onclick="return confirm('¿Eliminar {{ .Target.Name }}?');">Eliminar</button>
				</form>
			  </details>
			</td>
		  </tr>
		  {{- end }}
		</tbody>
	  </table>
	  <p class="footer">API disponible en <a href="/api/status">/api/status</a></p>
	</section>
  </main>
</body>
</html>
`
