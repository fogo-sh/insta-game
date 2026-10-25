package lobby

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type memoryPlacements struct {
	rows        map[string]Placement
	afterAssign func()
}

func (p *memoryPlacements) Assign(_ context.Context, next Placement) (string, error) {
	if old, ok := p.rows[next.ID]; ok {
		if old.Game != next.Game || old.Name != next.Name {
			return "", ErrPlacementConflict
		}
		return old.Host, nil
	}
	p.rows[next.ID] = next
	if p.afterAssign != nil {
		p.afterAssign()
	}
	return next.Host, nil
}
func postGame(f *Fleet, payload string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/games", strings.NewReader(payload))
	r.Header.Set("Authorization", "Bearer invitation")
	w := httptest.NewRecorder()
	f.Handler().ServeHTTP(w, r)
	return w
}
func TestAutomaticPlacementKeepsItsOwnerAfterAnInterruptedCreate(t *testing.T) {
	a, ha, sa := setup()
	b, hb, sb := setup()
	ha.state, hb.state = "running", "running"
	for _, m := range []*memoryTable{a, b} {
		m.rows["quake"] = Row{Snapshot: Snapshot{ID: "quake", Name: "Preset", Game: "Quake", Desired: "stopped", Phase: "stopped", ObservedAt: time.Now()}}
	}
	a.rows["busy"] = Row{TemplateID: "quake", Snapshot: Snapshot{ID: "busy", Game: "Quake", Desired: "running", ObservedAt: time.Now()}}
	assignments := &memoryPlacements{rows: map[string]Placement{}, afterAssign: func() { b.err = errors.New("temporary write failure") }}
	f := &Fleet{TokenHash: sa.TokenHash, Placements: assignments, Members: []Member{{ID: "a", Name: "A", Service: sa}, {ID: "b", Name: "B", Service: sb}}}
	payload := `{"id":"server-11111111111111111111111111111111","game":"Quake","name":"Friday"}`
	if w := postGame(f, payload); w.Code != 503 {
		t.Fatal(w.Code, w.Body)
	}
	if assignments.rows["server-11111111111111111111111111111111"].Host != "b" {
		t.Fatal("did not pick the least busy host")
	}
	b.err = nil
	assignments.afterAssign = nil
	for _, id := range []string{"busy-1", "busy-2"} {
		b.rows[id] = Row{TemplateID: "quake", Snapshot: Snapshot{ID: id, Game: "Quake", Desired: "running", ObservedAt: time.Now()}}
	}
	if w := postGame(f, payload); w.Code != 202 {
		t.Fatal(w.Code, w.Body)
	}
	id := "server-11111111111111111111111111111111"
	if a.rows[id].Snapshot.ID != "" || b.rows[id].Snapshot.Name != "Friday" || ha.wakes != 0 || hb.wakes != 1 {
		t.Fatal("retry moved or duplicated the server")
	}
	if w := postGame(f, strings.Replace(payload, "Friday", "Saturday", 1)); w.Code != 409 {
		t.Fatal("reused server ID accepted", w.Code)
	}
}
