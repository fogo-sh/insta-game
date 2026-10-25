package lobby

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestFleetRoutesIdenticalGameIDsToTheirOwnHosts(t *testing.T) {
	a, ha, sa := setup()
	b, hb, sb := setup()
	for _, m := range []*memoryTable{a, b} {
		m.rows["quake"] = Row{Snapshot: Snapshot{ID: "quake", Name: "Quake", Phase: "stopped", ObservedAt: time.Now()}}
	}
	f := &Fleet{TokenHash: sa.TokenHash, Members: []Member{{ID: "a", Name: "A", Service: sa}, {ID: "b", Name: "B", Service: sb}}}
	handler := f.Handler()
	if w := request(handler, "GET", "/api/games", ""); w.Code != 401 {
		t.Fatal("unprotected fleet", w.Code)
	}
	if w := request(handler, "POST", "/api/hosts/b/games/quake/start", "invitation"); w.Code != 202 {
		t.Fatal(w.Code, w.Body)
	}
	if ha.wakes != 0 || hb.wakes != 1 || a.rows["quake"].Pending(time.Now()) || !b.rows["quake"].Pending(time.Now()) {
		t.Fatal("request crossed host boundary")
	}
	if w := request(handler, "POST", "/api/hosts/missing/games/quake/start", "invitation"); w.Code != 404 {
		t.Fatal("unknown host accepted", w.Code)
	}
	w := request(handler, "GET", "/api/games", "invitation")
	var listing struct {
		Hosts   []map[string]string `json:"hosts"`
		Games   []View              `json:"games"`
		Presets []map[string]string `json:"presets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Hosts) != 2 || len(listing.Games) != 2 || len(listing.Presets) != 2 || listing.Games[0].HostID == listing.Games[1].HostID {
		t.Fatalf("lost namespaced inventory: %+v", listing)
	}
}

func TestFleetKeepsHealthyHostsVisibleAndRetriesEveryHost(t *testing.T) {
	a, ha, sa := setup()
	b, hb, sb := setup()
	ha.err = errors.New("host failed")
	for _, m := range []*memoryTable{a, b} {
		m.rows["quake"] = Row{Snapshot: Snapshot{ID: "quake", Phase: "stopped"}}
		if err := m.Queue(context.Background(), "quake", "start", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	f := &Fleet{TokenHash: sa.TokenHash, Members: []Member{{ID: "a", Name: "A", Service: sa}, {ID: "b", Name: "B", Service: sb}}}
	w := request(f.Handler(), "GET", "/api/games", "invitation")
	var body struct {
		Hosts []map[string]string `json:"hosts"`
		Games []View              `json:"games"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(body.Hosts) != 2 || body.Hosts[0]["error"] == "" || len(body.Games) != 1 || body.Games[0].HostID != "b" {
		t.Fatal("one failed host hid healthy inventory", w.Body)
	}
	if err := f.Reconcile(context.Background()); err == nil {
		t.Fatal("failed host retry was not reported")
	}
	if ha.wakes != 1 || hb.wakes != 1 {
		t.Fatal("retry skipped a host", ha.wakes, hb.wakes)
	}
}
