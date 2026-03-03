package routing

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	kedacache "github.com/kedacore/http-add-on/pkg/cache"
	"github.com/kedacore/http-add-on/pkg/queue"
)

func newTestClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(kedacache.NewScheme()).
		WithObjects(objs...).
		Build()
}

func startTableAndWaitForSync(t *testing.T, tbl Table) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_ = tbl.Start(ctx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tbl.HasSynced() {
			return cancel
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	t.Fatal("table did not sync within timeout")
	return cancel
}

func TestTableRoute_NilRequest(t *testing.T) {
	cl := newTestClient()
	tbl := NewTable(cl, queue.NewMemory())

	cancel := startTableAndWaitForSync(t, tbl)
	defer cancel()

	result := tbl.Route(nil)
	if result != nil {
		t.Error("expected nil for nil request")
	}
}

func TestTableRoute(t *testing.T) {
	first := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "first", Namespace: "default"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"first.example.com"}},
	}
	second := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "second", Namespace: "default"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"second.example.com"}},
	}

	tests := map[string][]*httpv1alpha1.HTTPScaledObject{
		"single host":    {first},
		"multiple hosts": {first, second},
	}

	for name, httpsos := range tests {
		t.Run(name, func(t *testing.T) {
			// Convert the HTTPSO slice to a client.Object slice for creating a test client
			objs := make([]client.Object, len(httpsos))
			for i, h := range httpsos {
				objs[i] = h
			}

			cl := newTestClient(objs...)
			tbl := NewTable(cl, queue.NewMemory())

			cancel := startTableAndWaitForSync(t, tbl)
			defer cancel()

			for _, httpso := range httpsos {
				host := httpso.Spec.Hosts[0]
				req, _ := http.NewRequest("GET", "http://"+host+"/test", nil)
				if result := tbl.Route(req); result == nil || result.Name != httpso.Name {
					t.Errorf("host %q: expected %q, got %v", host, httpso.Name, result)
				}
			}

			unknownReq, _ := http.NewRequest("GET", "http://unknown.example.com/test", nil)
			if tbl.Route(unknownReq) != nil {
				t.Error("expected nil for unknown host")
			}
		})
	}
}

func TestTableHasSynced(t *testing.T) {
	cl := newTestClient()
	tbl := NewTable(cl, queue.NewMemory())

	// Initially not synced
	if tbl.HasSynced() {
		t.Error("expected HasSynced to be false initially")
	}

	cancel := startTableAndWaitForSync(t, tbl)
	defer cancel()

	// Now synced
	if !tbl.HasSynced() {
		t.Error("expected HasSynced to be true after sync")
	}
}

func TestTableHealthCheck(t *testing.T) {
	cl := newTestClient()
	tbl := NewTable(cl, queue.NewMemory())

	ctx := context.Background()

	// Initially not synced, health check should fail
	err := tbl.HealthCheck(ctx)
	if err == nil {
		t.Error("expected error when not synced")
	}
	if !errors.Is(err, errNotSyncedTable) {
		t.Errorf("expected errNotSyncedTable, got: %v", err)
	}

	cancel := startTableAndWaitForSync(t, tbl)
	defer cancel()

	// Now healthy
	err = tbl.HealthCheck(ctx)
	if err != nil {
		t.Errorf("expected no error when synced, got: %v", err)
	}
}

func TestTableSignal_TriggersRefresh(t *testing.T) {
	httpso := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "initial",
			Namespace: "default",
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			Hosts: []string{"initial.example.com"},
		},
	}

	cl := newTestClient(httpso)
	tbl := NewTable(cl, queue.NewMemory())

	cancel := startTableAndWaitForSync(t, tbl)
	defer cancel()

	// Verify initial routing works
	req, _ := http.NewRequest("GET", "http://initial.example.com/test", nil)
	result := tbl.Route(req)
	if result == nil || result.Name != "initial" {
		t.Fatal("expected to route to 'initial'")
	}

	// Add a new object directly to the fake client
	newHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new-object",
			Namespace: "default",
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			Hosts: []string{"new.example.com"},
		},
	}
	if err := cl.Create(context.Background(), newHTTPSO); err != nil {
		t.Fatalf("failed to create new HTTPScaledObject: %v", err)
	}

	// Signal to trigger refresh
	tbl.Signal()

	// Wait for the new object to be picked up
	deadline := time.Now().Add(5 * time.Second)
	var newResult *httpv1alpha1.HTTPScaledObject
	newReq, _ := http.NewRequest("GET", "http://new.example.com/test", nil)
	for time.Now().Before(deadline) {
		newResult = tbl.Route(newReq)
		if newResult != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if newResult == nil {
		t.Fatal("expected to route to new object after Signal")
	}
	if newResult.Name != "new-object" {
		t.Errorf("expected name 'new-object', got %q", newResult.Name)
	}
}

func TestTableRefreshMemory_CancelsOnContextDone(t *testing.T) {
	cl := newTestClient()
	tbl := NewTable(cl, queue.NewMemory())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- tbl.Start(ctx)
	}()

	// Wait for initial sync
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tbl.HasSynced() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cancel context
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("refreshMemory did not exit after context cancellation")
	}
}

func TestTableQueueCounterIntegration(t *testing.T) {
	httpso := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-httpso",
			Namespace: "default",
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			Hosts: []string{"example.com"},
			ScalingMetric: &httpv1alpha1.ScalingMetricSpec{
				Rate: &httpv1alpha1.RateMetricSpec{
					Window:      metav1.Duration{Duration: 2 * time.Minute},
					Granularity: metav1.Duration{Duration: 2 * time.Second},
				},
			},
		},
	}

	cl := newTestClient(httpso)
	counter := queue.NewMemory()
	tbl := NewTable(cl, counter)

	cancel := startTableAndWaitForSync(t, tbl)
	defer cancel()

	// Verify the queue counter registered the key
	key := "default/test-httpso"
	counts, err := counter.Current()
	if err != nil {
		t.Fatalf("failed to get current counts: %v", err)
	}
	if _, exists := counts.Counts[key]; !exists {
		t.Errorf("expected queue counter to have key %q", key)
	}
}

func TestTableSignal_DeletedObjectBecomesUnroutable(t *testing.T) {
	httpso := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "to-delete",
			Namespace: "default",
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			Hosts: []string{"delete.example.com"},
		},
	}

	cl := newTestClient(httpso)
	counter := queue.NewMemory()
	tbl := NewTable(cl, counter)

	cancel := startTableAndWaitForSync(t, tbl)
	defer cancel()

	// Verify initial routing works
	req, _ := http.NewRequest("GET", "http://delete.example.com/test", nil)
	result := tbl.Route(req)
	if result == nil || result.Name != "to-delete" {
		t.Fatal("expected to route to 'to-delete'")
	}

	// Verify key exists in counter
	key := "default/to-delete"
	counts, err := counter.Current()
	if err != nil {
		t.Fatalf("failed to get current counts: %v", err)
	}
	if _, exists := counts.Counts[key]; !exists {
		t.Fatalf("expected queue counter to have key %q before deletion", key)
	}

	// Delete the object
	if err := cl.Delete(context.Background(), httpso); err != nil {
		t.Fatalf("failed to delete HTTPScaledObject: %v", err)
	}

	// Signal to trigger refresh
	tbl.Signal()

	// Wait for the deletion to be picked up
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tbl.Route(req) == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify object is no longer routable
	if tbl.Route(req) != nil {
		t.Error("expected deleted object to be unroutable")
	}

	// Verify key was removed from counter
	counts, err = counter.Current()
	if err != nil {
		t.Fatalf("failed to get current counts: %v", err)
	}
	if _, exists := counts.Counts[key]; exists {
		t.Errorf("expected queue counter to NOT have key %q after deletion", key)
	}
}
