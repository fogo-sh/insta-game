package fleet

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu         sync.Mutex
	state      State
	reports    map[string]Report
	tombstones map[string]bool
	beforeSave func(State)
	publishErr error
}

func newMemoryStore() *memoryStore {
	return &memoryStore{reports: map[string]Report{}, tombstones: map[string]bool{}}
}
func cloneState(s State) State {
	s.Hosts = append([]Host(nil), s.Hosts...)
	s.Servers = append([]Server(nil), s.Servers...)
	return s
}
func (m *memoryStore) Load(context.Context) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneState(m.state), nil
}
func (m *memoryStore) Save(_ context.Context, s State, previous int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.beforeSave != nil {
		m.beforeSave(s)
	}
	if m.state.Revision != previous {
		return ErrConflict
	}
	s.Revision = previous + 1
	m.state = cloneState(s)
	return nil
}
func (m *memoryStore) Reports(context.Context) ([]Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Report
	for _, r := range m.reports {
		result = append(result, r)
	}
	return result, nil
}
func (m *memoryStore) Publish(_ context.Context, r Report) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.publishErr != nil {
		return m.publishErr
	}
	if !m.tombstones[r.ID] {
		m.reports[r.ID] = r
	}
	return nil
}
func (m *memoryStore) Tombstoned(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tombstones[id], nil
}
func (m *memoryStore) Forget(_ context.Context, s State, previous int64, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if previous != m.state.Revision {
		return ErrConflict
	}
	s.Revision = previous + 1
	m.state = cloneState(s)
	m.tombstones[id] = true
	delete(m.reports, id)
	return nil
}

type fakeMachines struct {
	mu          sync.Mutex
	states      map[string]string
	wakes       int
	stops       int
	deletes     int
	deleteError error
}

func (f *fakeMachines) Inspect(_ context.Context, h Host) (Machine, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	state := f.states[h.ID]
	if state == "" {
		state = "new"
	}
	return Machine{ID: h.InstanceID, State: state}, nil
}
func (f *fakeMachines) Wake(_ context.Context, h Host) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.wakes++
	return "i-" + h.ID, nil
}
func (f *fakeMachines) Delete(context.Context, Host) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes++
	return f.deleteError
}
func (f *fakeMachines) Stop(context.Context, Host) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	return nil
}

func testScheduler() (*Scheduler, *memoryStore, *fakeMachines) {
	store := newMemoryStore()
	machines := &fakeMachines{states: map[string]string{}}
	config := Config{Games: []Game{{ID: "quake", Name: "Quake", Resources: Resources{CPU: 100, MemoryMiB: 256, DiskMiB: 1024}}}, Capacity: Resources{CPU: 200, MemoryMiB: 512, DiskMiB: 2048}, MaxHosts: 2, MaxServers: 8, LaunchTemplate: "lt-example", LaunchVersion: "1"}
	return &Scheduler{Store: store, Machines: machines, Config: config}, store, machines
}

func TestConcurrentCreationReservesCapacityAndDuplicateIDOnce(t *testing.T) {
	scheduler, store, _ := testScheduler()
	ctx := context.Background()
	var group sync.WaitGroup
	for i := range 12 {
		group.Go(func() {
			_, err := scheduler.Create(ctx, fmt.Sprintf("server-%d", i%4), "quake", "Friday")
			if err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	state, _ := store.Load(ctx)
	if len(state.Hosts) != 2 || len(state.Servers) != 4 {
		t.Fatalf("allocated %d hosts / %d servers", len(state.Hosts), len(state.Servers))
	}
	for _, h := range state.Hosts {
		if !fits(h.Capacity, used(state, h.ID), Resources{}) {
			t.Fatal("overcommitted a host")
		}
	}
	if err := scheduler.Action(ctx, state.Servers[0].ID, "stop"); err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Create(ctx, "extra", "quake", "Extra"); !errors.Is(err, ErrCapacity) {
		t.Fatalf("stopping must retain reservation: %v", err)
	}
}

func TestRetirementCannotDeleteConcurrentAssignment(t *testing.T) {
	scheduler, store, machines := testScheduler()
	store.state = State{Revision: 1, Hosts: []Host{{ID: "host-one", Capacity: scheduler.Config.Capacity}}}
	store.beforeSave = func(candidate State) {
		if !candidate.Hosts[0].Retiring {
			return
		}
		store.beforeSave = nil
		store.state.Revision++
		store.state.Servers = []Server{{ID: "server-one", Host: "host-one", Game: "quake", Desired: "running", RequestID: "start", Resources: scheduler.Config.Games[0].Resources}}
	}
	if err := scheduler.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, _ := store.Load(context.Background())
	if machines.deletes != 0 || state.Hosts[0].Retiring || len(state.Servers) != 1 {
		t.Fatal("retirement raced past a saved assignment")
	}
}

func TestIdleStopAcknowledgementDoesNotWakeOrOverrideNewStart(t *testing.T) {
	scheduler, store, machines := testScheduler()
	ctx := context.Background()
	server, err := scheduler.Create(ctx, "server-one", "quake", "Friday")
	if err != nil {
		t.Fatal(err)
	}
	report := Report{ID: server.ID, Host: server.Host, RequestID: server.RequestID, Desired: "stopped", Phase: "stopped", ObservedAt: time.Now()}
	if err := store.Publish(ctx, report); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ := store.Load(ctx)
	if machines.wakes != 0 || state.Servers[0].Desired != "stopped" || state.Servers[0].RequestID != server.RequestID {
		t.Fatal("idle stop was replayed or woke its host")
	}
	if err := scheduler.Action(ctx, server.ID, "start"); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Load(ctx)
	if machines.wakes != 1 || state.Servers[0].Desired != "running" || state.Servers[0].RequestID == server.RequestID {
		t.Fatal("old idle report undid a newer start")
	}
}

func TestRemovalRequiresSuccessfulCurrentAcknowledgementAndRetainsSlotUntilTermination(t *testing.T) {
	scheduler, store, machines := testScheduler()
	ctx := context.Background()
	server, err := scheduler.Create(ctx, "server-one", "quake", "Friday")
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := scheduler.Create(ctx, "server-retained", "quake", "Retained")
	if err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Action(ctx, sibling.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Action(ctx, server.ID, "remove"); err != nil {
		t.Fatal(err)
	}
	state, _ := store.Load(ctx)
	current := state.Servers[0]
	report := Report{ID: current.ID, Host: current.Host, RequestID: current.RequestID, Desired: "deleted", Phase: "deleted", Error: "Delete failed", ObservedAt: time.Now()}
	_ = store.Publish(ctx, report)
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Load(ctx)
	if len(state.Servers) != 2 || machines.deletes != 0 {
		t.Fatal("failed removal freed reservation")
	}
	report.Error = ""
	_ = store.Publish(ctx, report)
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Load(ctx)
	if len(state.Servers) != 1 || !store.tombstones[server.ID] {
		t.Fatal("removal did not archive server")
	}
	if err := scheduler.Action(ctx, sibling.ID, "remove"); err != nil {
		t.Fatal(err)
	}
	machines.deleteError = errors.New("termination in progress")
	if err := scheduler.Reconcile(ctx); err == nil {
		t.Fatal("termination failure hidden")
	}
	state, _ = store.Load(ctx)
	if len(state.Hosts) != 1 || !state.Hosts[0].Retiring {
		t.Fatal("released an unterminated host slot")
	}
	machines.deleteError = nil
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Load(ctx)
	if len(state.Hosts) != 0 {
		t.Fatal("terminated host slot retained")
	}
	if _, err := scheduler.Create(ctx, server.ID, "quake", "Friday"); err == nil {
		t.Fatal("delayed retry resurrected deleted server")
	}
}

func TestExpiryRemovesRecordsOnlyAfterOwnedMachineTermination(t *testing.T) {
	scheduler, store, machines := testScheduler()
	ctx := context.Background()
	server, err := scheduler.Create(ctx, "server-one", "quake", "Friday")
	if err != nil {
		t.Fatal(err)
	}
	scheduler.Config.Deadline = time.Now().Add(-time.Minute).Unix()
	machines.deleteError = errors.New("still terminating")
	if err := scheduler.Reconcile(ctx); err == nil {
		t.Fatal("termination failure hidden")
	}
	state, _ := store.Load(ctx)
	if !state.Deleting || len(state.Hosts) != 1 || len(state.Servers) != 1 {
		t.Fatal("expiry forgot data before termination")
	}
	machines.deleteError = nil
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Load(ctx)
	if len(state.Hosts) != 0 || len(state.Servers) != 0 || !store.tombstones[server.ID] {
		t.Fatal("expiry left orphan allocations")
	}
}

func TestStartupDeadlineStopsEmptyHostWithoutRemovingDataAndAllowsRetry(t *testing.T) {
	scheduler, store, machines := testScheduler()
	ctx := context.Background()
	server, err := scheduler.Create(ctx, "server-one", "quake", "Friday")
	if err != nil {
		t.Fatal(err)
	}
	store.state.Servers[0].RequestedAt = time.Now().Add(-StartupTimeout - time.Minute).Unix()
	for range 2 {
		if err := scheduler.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
	}
	state, _ := store.Load(ctx)
	if machines.wakes != 0 || machines.stops != 2 || machines.deletes != 0 || len(state.Servers) != 1 || state.Servers[0].Error != StartupTimeoutMessage {
		t.Fatal("expired startup was relaunched, forgotten, or not stopped")
	}
	if err := scheduler.Action(ctx, server.ID, "start"); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Load(ctx)
	if machines.wakes != 1 || state.Servers[0].RequestID == server.RequestID || state.Servers[0].Error != "" {
		t.Fatal("explicit retry did not renew the startup deadline")
	}
}

func TestExpiredStartupNeverStopsHealthySiblingOrAgesOutReadyServer(t *testing.T) {
	scheduler, store, machines := testScheduler()
	ctx := context.Background()
	healthy, err := scheduler.Create(ctx, "healthy", "quake", "Healthy")
	if err != nil {
		t.Fatal(err)
	}
	players := 1
	_ = store.Publish(ctx, Report{ID: healthy.ID, Host: healthy.Host, RequestID: healthy.RequestID, Desired: "running", Phase: "running", Players: &players, ObservedAt: time.Now()})
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Create(ctx, "failed", "quake", "Failed"); err != nil {
		t.Fatal(err)
	}
	for i := range store.state.Servers {
		store.state.Servers[i].RequestedAt = time.Now().Add(-time.Hour).Unix()
	}
	machines.wakes = 0
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ := store.Load(ctx)
	if machines.stops != 0 || machines.wakes != 1 || state.Servers[0].ReadyAt == 0 || state.Servers[0].Error != "" || state.Servers[1].Error == "" {
		t.Fatal("startup cutoff interrupted a healthy shared host")
	}
}

func TestUnacknowledgedStopAndRemoveDoNotWakeForever(t *testing.T) {
	for _, action := range []string{"stop", "remove"} {
		t.Run(action, func(t *testing.T) {
			scheduler, store, machines := testScheduler()
			ctx := context.Background()
			server, err := scheduler.Create(ctx, "server-one", "quake", "Friday")
			if err != nil {
				t.Fatal(err)
			}
			sibling, err := scheduler.Create(ctx, "server-retained", "quake", "Retained")
			if err != nil {
				t.Fatal(err)
			}
			if err := scheduler.Action(ctx, sibling.ID, "stop"); err != nil {
				t.Fatal(err)
			}
			retained := store.state.Servers[1]
			_ = store.Publish(ctx, Report{ID: retained.ID, Host: retained.Host, RequestID: retained.RequestID, Phase: "stopped", Desired: "stopped", ObservedAt: time.Now()})
			if err := scheduler.Action(ctx, server.ID, action); err != nil {
				t.Fatal(err)
			}
			store.state.Servers[0].RequestedAt = time.Now().Add(-StartupTimeout - time.Minute).Unix()
			old := store.state.Servers[0].RequestID
			if err := scheduler.Reconcile(ctx); err != nil {
				t.Fatal(err)
			}
			if machines.wakes != 0 || machines.stops != 1 || machines.deletes != 0 || len(store.state.Servers) != 2 || store.state.Servers[0].Error != RequestTimeoutMessage {
				t.Fatal("expired control action spent indefinitely or discarded saved data")
			}
			if err := scheduler.Action(ctx, server.ID, action); err != nil {
				t.Fatal(err)
			}
			if store.state.Servers[0].RequestID == old || store.state.Servers[0].Error != "" {
				t.Fatal("explicit action retry not accepted")
			}
		})
	}
}

func TestDeletingEveryAssignmentCleansFailedHostWithoutWorker(t *testing.T) {
	scheduler, store, machines := testScheduler()
	ctx := context.Background()
	first, err := scheduler.Create(ctx, "server-first", "quake", "First")
	if err != nil {
		t.Fatal(err)
	}
	second, err := scheduler.Create(ctx, "server-second", "quake", "Second")
	if err != nil {
		t.Fatal(err)
	}
	other, err := scheduler.Create(ctx, "server-other", "quake", "Other host")
	if err != nil {
		t.Fatal(err)
	}
	if first.Host != second.Host || first.Host == other.Host {
		t.Fatal("invalid test placement")
	}
	for _, id := range []string{first.ID, second.ID} {
		if err := scheduler.Action(ctx, id, "remove"); err != nil {
			t.Fatal(err)
		}
	}
	machines.deleteError = errors.New("waiting for disk deletion")
	if err := scheduler.Reconcile(ctx); err == nil {
		t.Fatal("termination error hidden")
	}
	state, _ := store.Load(ctx)
	if len(state.Servers) != 3 || len(state.Hosts) != 2 || len(store.tombstones) != 0 {
		t.Fatal("capacity released before physical deletion")
	}
	machines.deleteError = nil
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Load(ctx)
	if len(state.Servers) != 1 || state.Servers[0].ID != other.ID || len(state.Hosts) != 1 || state.Hosts[0].ID != other.Host || !store.tombstones[first.ID] || !store.tombstones[second.ID] || store.tombstones[other.ID] {
		t.Fatal("failed-host cleanup forgot the wrong assignments")
	}
}

func TestReleaseUpdateDoesNotPlaceNewServerOnOldHost(t *testing.T) {
	scheduler, store, _ := testScheduler()
	ctx := context.Background()
	scheduler.Config.Release = "releases/old"
	old, err := scheduler.Create(ctx, "server-old", "quake", "Old")
	if err != nil {
		t.Fatal(err)
	}
	scheduler.Config.Release = "releases/new"
	current, err := scheduler.Create(ctx, "server-new", "quake", "New")
	if err != nil {
		t.Fatal(err)
	}
	if old.Host == current.Host || store.state.Hosts[0].Release != "releases/old" || store.state.Hosts[1].Release != "releases/new" {
		t.Fatal("release update reused an incompatible pinned host")
	}
}

func TestCompatiblePageUpdateReusesPinnedHost(t *testing.T) {
	scheduler, store, _ := testScheduler()
	scheduler.Config.Release = "release-a"
	scheduler.Config.Workload = "same-host-workload"
	first, err := scheduler.Create(context.Background(), "first", "quake", "First")
	if err != nil {
		t.Fatal(err)
	}
	scheduler.Config.Release = "release-b"
	second, err := scheduler.Create(context.Background(), "second", "quake", "Second")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := store.Load(context.Background())
	if second.Host != first.Host || len(state.Hosts) != 1 || state.Hosts[0].Release != "release-a" {
		t.Fatal("page update stranded reusable host or changed its boot provenance", state)
	}
}
