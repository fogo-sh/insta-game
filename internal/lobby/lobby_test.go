package lobby

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fogo-sh/insta-game/internal/control"
)

type memoryTable struct {
	mu   sync.Mutex
	rows map[string]Row
	err  error
}

func (m *memoryTable) Read(_ context.Context, id string) (Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rows[id], m.err
}
func (m *memoryTable) Request(ctx context.Context, id string, now time.Time) error {
	return m.Queue(ctx, id, "start", now)
}
func (m *memoryTable) Queue(_ context.Context, id, action string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rows[id]
	if r.Pending(now) || (r.Action == action && r.RequestedAt > now.Add(-30*time.Second).Unix()) {
		return ErrBusy
	}
	r.Action = action
	r.RequestID = control.NewToken()
	r.RequestedAt = now.Unix()
	r.ExpiresAt = now.Add(RequestLifetime).Unix()
	r.Failure = ""
	m.rows[id] = r
	return m.err
}
func (m *memoryTable) Publish(_ context.Context, s Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rows[s.ID]
	r.Snapshot = s
	m.rows[s.ID] = r
	return m.err
}
func (m *memoryTable) Mark(_ context.Context, id, req, stage, failure string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rows[id]
	if r.RequestID == req {
		if stage == "acceptedID" {
			r.AcceptedID = req
		} else {
			r.CompletedID = req
			r.Failure = failure
		}
		m.rows[id] = r
	}
	return m.err
}

type fakeHost struct {
	state string
	wakes int
	err   error
}

func (h *fakeHost) State(context.Context) (string, error) { return h.state, h.err }
func (h *fakeHost) Wake(context.Context) error            { h.wakes++; return h.err }
func setup() (*memoryTable, *fakeHost, *Service) {
	sum := sha256.Sum256([]byte("invitation"))
	m := &memoryTable{rows: map[string]Row{}}
	h := &fakeHost{state: "stopped"}
	return m, h, &Service{Table: m, Host: h, IDs: []string{"quake"}, TokenHash: hex.EncodeToString(sum[:])}
}
func request(handler http.Handler, method, path, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestLobbyKeepsAdminAndInvitationSeparate(t *testing.T) {
	m, h, s := setup()
	m.rows["quake"] = Row{Snapshot: Snapshot{ID: "quake", Name: "Friday", Phase: "stopped"}}
	handler := s.Handler()
	for _, path := range []string{"/api/games", "/api/games/quake/start"} {
		if w := request(handler, "POST", path, "controller-token"); w.Code != 401 {
			t.Fatalf("admin token accepted: %d", w.Code)
		}
	}
	if w := request(handler, "POST", "/api/games/other/start", "invitation"); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := request(handler, "POST", "/api/games/quake/start", "invitation"); w.Code != 202 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(handler, "POST", "/api/games/quake/start", "invitation"); w.Code != 409 {
		t.Fatal(w.Code, w.Body)
	}
	if h.wakes != 1 || !m.rows["quake"].Pending(time.Now()) {
		t.Fatal("start not durably recorded")
	}
	if w := request(handler, "DELETE", "/api/instances/quake", "invitation"); w.Code != 404 && w.Code != 405 {
		t.Fatalf("unexpected admin API: %d", w.Code)
	}
}
func TestLobbyHidesStaleAndSleepingAddresses(t *testing.T) {
	m, h, s := setup()
	players := 2
	for _, test := range []struct {
		state   string
		age     time.Duration
		visible bool
	}{{"stopped", 0, false}, {"running", time.Minute, false}, {"running", 0, true}} {
		h.state = test.state
		m.rows["quake"] = Row{Snapshot: Snapshot{ID: "quake", Phase: "running", Address: "203.0.113.1:26000", Players: &players, ObservedAt: time.Now().Add(-test.age)}}
		w := request(s.Handler(), "GET", "/api/games", "invitation")
		if w.Code != 200 || strings.Contains(w.Body.String(), "203.0.113.1") != test.visible {
			t.Fatal(w.Code, w.Body)
		}
	}
}
func TestPendingWakeSurvivesFailedInitialAttemptAndExpires(t *testing.T) {
	m, h, s := setup()
	m.rows["quake"] = Row{Snapshot: Snapshot{ID: "quake", Phase: "stopped"}}
	h.err = errors.New("AWS unavailable")
	w := request(s.Handler(), "POST", "/api/games/quake/start", "invitation")
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	h.err = nil
	if err := s.Reconcile(context.Background()); err != nil || h.wakes != 2 {
		t.Fatal(err, h.wakes)
	}
	r := m.rows["quake"]
	r.ExpiresAt = time.Now().Add(-time.Second).Unix()
	m.rows["quake"] = r
	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.wakes != 2 || m.rows["quake"].Failure == "" {
		t.Fatal("expired request woke the host")
	}
}
func TestWorkerConsumesOnceAndDoesNotRestartAnIdleGame(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	m, _, _ := setup()
	store, err := control.OpenStore(filepath.Join(t.TempDir(), "instances.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.Create(control.Instance{ID: "quake", Name: "Friday", Game: control.Game{ID: "quake1", DisplayName: "Quake"}, Desired: "stopped", Phase: "stopped", UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	c := control.NewController(store, nil, nil, "")
	w := &Worker{Table: m, IDs: []string{"quake"}, Controller: c, StartedAt: now}
	if err := w.Sync(ctx, now); err != nil {
		t.Fatal(err)
	}
	if err := m.Request(ctx, "quake", now); err != nil {
		t.Fatal(err)
	}
	if err := w.Sync(ctx, now); err != nil {
		t.Fatal(err)
	}
	r := m.rows["quake"]
	if r.AcceptedID != r.RequestID || r.CompletedID == r.RequestID {
		t.Fatal("request not accepted separately from readiness")
	}
	if pending, err := w.Pending(ctx, now); err != nil || !pending {
		t.Fatal(pending, err)
	}
	if err := store.Update("quake", func(i *control.Instance) error {
		i.Phase = "running"
		i.Players = new(int)
		i.UpdatedAt = now.Add(time.Second)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Sync(ctx, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if pending, err := w.Pending(ctx, now); err != nil || pending {
		t.Fatal(pending, err)
	}
	if err := c.SetDesired("quake", "stopped"); err != nil {
		t.Fatal(err)
	}
	if err := w.Sync(ctx, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	i, _ := store.Get("quake")
	if i.Desired != "stopped" {
		t.Fatal("completed request restarted an idle game")
	}
	m.err = errors.New("table unreachable")
	if _, err := w.Pending(ctx, now); err == nil {
		t.Fatal("storage outage must prevent idle shutdown")
	}
}
func TestWorkerLeavesRequestPendingAfterShutdownCommits(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	m, _, _ := setup()
	store, err := control.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.Create(control.Instance{ID: "quake", Desired: "stopped", Phase: "stopped"})
	c := control.NewController(store, nil, nil, "")
	committed, err := c.BeginIdleShutdown()
	if err != nil || !committed {
		t.Fatal(committed, err)
	}
	m.rows["quake"] = Row{RequestID: "new", ExpiresAt: now.Add(time.Minute).Unix()}
	w := &Worker{Table: m, IDs: []string{"quake"}, Controller: c}
	if err := w.Sync(ctx, now); err != nil {
		t.Fatal(err)
	}
	if m.rows["quake"].AcceptedID != "" || !m.rows["quake"].Pending(now) {
		t.Fatal("shutdown swallowed request")
	}
}

func TestAcceptedRequestSurvivesLostAcknowledgementAndControllerRestart(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	m, _, _ := setup()
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := control.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(control.Instance{ID: "quake", Desired: "stopped", Phase: "stopped"}); err != nil {
		t.Fatal(err)
	}
	c := control.NewController(store, nil, nil, "")
	m.rows["quake"] = Row{RequestID: "durable-start", ExpiresAt: now.Add(time.Minute).Unix()}
	// Acceptance commits locally, then the process loses the remote acknowledgement.
	if err := c.RequestStart("quake", "durable-start"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDesired("quake", "stopped"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = control.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	c = control.NewController(store, nil, nil, "")
	w := &Worker{Table: m, IDs: []string{"quake"}, Controller: c}
	if err := w.Sync(ctx, now); err != nil {
		t.Fatal(err)
	}
	instance, err := store.Get("quake")
	if err != nil {
		t.Fatal(err)
	}
	if instance.Desired != "stopped" || m.rows["quake"].AcceptedID != "durable-start" {
		t.Fatal("replayed request undid a later stop")
	}
	if err := w.Sync(ctx, now); err != nil {
		t.Fatal(err)
	}
	if m.rows["quake"].CompletedID != "durable-start" {
		t.Fatal("stopped request never completed")
	}
}
func TestPreviousBootFailureDoesNotCompletePendingRequest(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	m, _, _ := setup()
	store, err := control.OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Create(control.Instance{ID: "quake", Desired: "running", Phase: "failed", UpdatedAt: now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	m.rows["quake"] = Row{RequestID: "boot", AcceptedID: "boot", ExpiresAt: now.Add(time.Minute).Unix()}
	w := &Worker{Table: m, IDs: []string{"quake"}, Controller: control.NewController(store, nil, nil, ""), StartedAt: now}
	if err := w.Sync(ctx, now); err != nil {
		t.Fatal(err)
	}
	if m.rows["quake"].CompletedID != "" {
		t.Fatal("old failure completed a new boot request")
	}
}

func (m *memoryTable) List(context.Context) ([]Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := []Row{}
	for _, r := range m.rows {
		rows = append(rows, r)
	}
	return rows, m.err
}
func (m *memoryTable) Create(_ context.Context, template string, s Snapshot, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rows[s.ID]; ok {
		return ErrBusy
	}
	m.rows[s.ID] = Row{TemplateID: template, Snapshot: s, Action: "start", RequestID: s.ID, ExpiresAt: now.Add(RequestLifetime).Unix(), RequestedAt: now.Unix()}
	return m.err
}

func TestIndependentServersFromOnePreset(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	table, host, service := setup()
	store, err := control.OpenStore(filepath.Join(t.TempDir(), "instances.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	game := control.Game{ID: "quake1", DisplayName: "Quake"}
	_, err = store.Create(control.Instance{ID: "quake", Name: "Preset", Game: game, Provider: "docker", Config: "private config", Desired: "stopped", Phase: "stopped", IdleMinutes: 15, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	controller := control.NewController(store, []control.Game{game}, map[string]control.Provider{"docker": nil}, "docker")
	worker := &Worker{Table: table, IDs: service.IDs, Controller: controller, StartedAt: now}
	if err := worker.Sync(ctx, now); err != nil {
		t.Fatal(err)
	}
	handler := service.Handler()
	ids := []string{"server-11111111111111111111111111111111", "server-22222222222222222222222222222222"}
	for _, id := range ids {
		payload := fmt.Sprintf(`{"id":%q,"template":"quake","name":"Friends"}`, id)
		for attempt := 0; attempt < 2; attempt++ {
			req := httptest.NewRequest("POST", "/api/games", strings.NewReader(payload))
			req.Header.Set("Authorization", "Bearer invitation")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != 202 {
				t.Fatal(response.Code, response.Body)
			}
		}
	}
	if err := worker.Sync(ctx, now); err != nil {
		t.Fatal(err)
	}
	first, _ := store.Get(ids[0])
	second, _ := store.Get(ids[1])
	preset, _ := store.Get("quake")
	if first.Config != "private config" || first.IdleMinutes != 15 || first.Desired != "running" || second.Desired != "running" || preset.Desired != "stopped" || first.AgentToken == second.AgentToken || first.RconPassword == second.RconPassword {
		t.Fatal("instances did not remain independent")
	}
	for _, id := range ids {
		if err := store.Update(id, func(i *control.Instance) error {
			i.Phase = "running"
			i.Players = new(int)
			i.UpdatedAt = now.Add(time.Second)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := worker.Sync(ctx, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := table.Queue(ctx, ids[0], "stop", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := worker.Sync(ctx, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	first, _ = store.Get(ids[0])
	second, _ = store.Get(ids[1])
	if first.Desired != "stopped" || second.Desired != "running" {
		t.Fatal("stop affected another instance")
	}
	if err := store.Update(ids[0], func(i *control.Instance) error {
		i.Phase = "stopped"
		i.UpdatedAt = now.Add(3 * time.Second)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := worker.Sync(ctx, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := table.Queue(ctx, ids[0], "remove", now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := worker.Sync(ctx, now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	row, _ := table.Read(ctx, ids[0])
	if !row.Pending(now) {
		t.Fatal("remove acknowledged before resources were deleted")
	}
	if err := store.Update(ids[0], func(i *control.Instance) error {
		i.Phase = "deleted"
		i.UpdatedAt = now.Add(5 * time.Second)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := worker.Sync(ctx, now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	row, _ = table.Read(ctx, ids[0])
	if row.Pending(now) || row.Failure != "" {
		t.Fatal("remove not completed", row)
	}
	if response := request(handler, "POST", "/api/games/quake/remove", "invitation"); response.Code != 409 {
		t.Fatal("preset deletion allowed", response.Code)
	}
	if response := request(handler, "POST", "/api/games/unrelated/stop", "invitation"); response.Code != 404 {
		t.Fatal("unrelated server exposed")
	}
	listing := request(handler, "GET", "/api/games", "invitation")
	var body struct {
		Presets []any
		Games   []View
	}
	if err := json.Unmarshal(listing.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Presets) != 1 || len(body.Games) != 3 || strings.Contains(listing.Body.String(), "private config") || host.wakes != 4 {
		t.Fatal(listing.Body.String(), host.wakes)
	}
}

func TestRemovedServersStayDeletedAfterHostRestart(t *testing.T) {
	table, _, service := setup()
	now := time.Now().UTC()
	store, err := control.OpenStore(filepath.Join(t.TempDir(), "instances.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id := "server-11111111111111111111111111111111"
	_, err = store.Create(control.Instance{ID: id, Name: "Removed", Desired: "deleted", Phase: "deleted", UpdatedAt: now.Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	table.rows[id] = Row{TemplateID: "quake", Snapshot: Snapshot{ID: id, Name: "Removed"}}
	worker := &Worker{Table: table, IDs: service.IDs, Controller: control.NewController(store, nil, nil, ""), StartedAt: now}
	if err := worker.Sync(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	response := request(service.Handler(), "GET", "/api/games", "invitation")
	var body struct{ Games []View }
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, game := range body.Games {
		if game.ID == id {
			if game.Phase != "deleted" {
				t.Fatal("removed server reappeared", game)
			}
			return
		}
	}
	t.Fatal("deleted status missing")
}
