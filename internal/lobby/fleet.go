package lobby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

type Binding struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	InstanceID string   `json:"instanceId"`
	StackID    string   `json:"stackId"`
	Table      string   `json:"table"`
	IDs        []string `json:"ids"`
}

func ParseBindings(value string) ([]Binding, error) {
	var bindings []Binding
	if err := json.Unmarshal([]byte(value), &bindings); err != nil {
		return nil, err
	}
	if len(bindings) == 0 || len(bindings) > 16 {
		return nil, fmt.Errorf("fleet requires 1–16 hosts")
	}
	seenIDs, seenInstances, seenTables := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, b := range bindings {
		if !instanceID.MatchString(b.ID) || strings.TrimSpace(b.Name) == "" || len(b.Name) > 80 || b.InstanceID == "" || b.StackID == "" || b.Table == "" || seenIDs[b.ID] || seenInstances[b.InstanceID] || seenTables[b.Table] {
			return nil, fmt.Errorf("fleet requires unique, named host bindings")
		}
		if _, err := ParseIDs(strings.Join(b.IDs, ",")); err != nil {
			return nil, err
		}
		seenIDs[b.ID], seenInstances[b.InstanceID], seenTables[b.Table] = true, true, true
	}
	return bindings, nil
}

type Member struct {
	ID, Name string
	Service  *Service
}
type Fleet struct {
	Members   []Member
	TokenHash string
}

func (f *Fleet) Reconcile(ctx context.Context) error {
	var failures []error
	for _, m := range f.Members {
		if err := m.Service.Reconcile(ctx); err != nil {
			failures = append(failures, fmt.Errorf("host %s: %w", m.ID, err))
		}
	}
	return errors.Join(failures...)
}
func (f *Fleet) Handler() http.Handler {
	mux := http.NewServeMux()
	files, _ := fs.Sub(web, "web")
	mux.Handle("GET /", http.FileServer(http.FS(files)))
	mux.HandleFunc("GET /api/games", func(w http.ResponseWriter, r *http.Request) {
		hosts := []map[string]string{}
		games := []View{}
		presets := []map[string]string{}
		for _, m := range f.Members {
			result, err := m.Service.view(r.Context())
			host := map[string]string{"id": m.ID, "name": m.Name, "state": result.Host}
			if err != nil {
				log.Printf("host %s: %v", m.ID, err)
				host["state"] = "unknown"
				host["error"] = "Status unavailable"
			} else {
				for _, g := range result.Games {
					g.HostID = m.ID
					games = append(games, g)
				}
				for _, p := range result.Presets {
					p["hostId"] = m.ID
					presets = append(presets, p)
				}
			}
			hosts = append(hosts, host)
		}
		respond(w, 200, map[string]any{"hosts": hosts, "games": games, "presets": presets})
	})
	mux.HandleFunc("POST /api/hosts/", http.NotFound)
	for _, m := range f.Members {
		prefix := "/api/hosts/" + m.ID
		handler := m.Service.routes()
		mux.Handle("POST "+prefix+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			request := r.Clone(r.Context())
			request.URL.Path = "/api" + strings.TrimPrefix(r.URL.Path, prefix)
			handler.ServeHTTP(w, request)
		}))
	}
	return protect(mux, f.TokenHash)
}
