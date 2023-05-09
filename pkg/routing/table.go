package routing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strings"
	"sync"

	kedahttp "github.com/kedacore/http-add-on/pkg/http"
)

// HostSwitch implements http.Handler. We need an object that implements the http.Handler interface.
// Therefore, we need a type for which we implement the ServeHTTP method.
// We just use a map here, in which we map host names (with port) to http.Handlers
type HostSwitch map[string]http.Handler

type TableReader interface {
	Lookup(string) (*Target, error)
	Hosts() []string
	HasHost(string) bool
}
type Table struct {
	fmt.Stringer
	m  map[string]Target
	Hs HostSwitch
	l  *sync.RWMutex
}

func NewTable() *Table {
	return &Table{
		m:  make(map[string]Target),
		Hs: make(HostSwitch),
		l:  new(sync.RWMutex),
	}
}

// Hosts is the TableReader implementation for t.
// This function returns all hosts that are currently
// in t.
func (t Table) Hosts() []string {
	t.l.RLock()
	defer t.l.RUnlock()
	ret := make([]string, 0, len(t.m))
	for host := range t.m {
		ret = append(ret, host)
	}
	return ret
}

func (t Table) HasHost(host string) bool {
	t.l.RLock()
	defer t.l.RUnlock()
	_, exists := t.m[host]
	return exists
}

func (t *Table) String() string {
	t.l.RLock()
	defer t.l.RUnlock()
	return fmt.Sprintf("%v", t.m)
}

func (t *Table) MarshalJSON() ([]byte, error) {
	t.l.RLock()
	defer t.l.RUnlock()
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(t.m)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (t *Table) UnmarshalJSON(data []byte) error {
	t.l.Lock()
	defer t.l.Unlock()
	t.m = map[string]Target{}
	b := bytes.NewBuffer(data)
	return json.NewDecoder(b).Decode(&t.m)
}

func (t *Table) Lookup(host string) (*Target, error) {
	t.l.RLock()
	defer t.l.RUnlock()

	keys := []string{host}
	if i := strings.LastIndex(host, ":"); i != -1 {
		// keys = append(keys, host[:i])
	}

	for _, key := range keys {
		if target, ok := t.m[key]; ok {
			return &target, nil
		}
	}

	return nil, ErrTargetNotFound
}

// AddTarget registers target for host in the routing table t
// if it didn't already exist.
//
// returns a non-nil error if it did already exist
func (t *Table) AddTarget(
	host string,
	target Target,
) error {
	t.l.Lock()
	defer t.l.Unlock()
	_, ok := t.m[host]
	if ok {
		return fmt.Errorf(
			"host %s is already registered in the routing table",
			host,
		)
	}
	t.m[host] = target
	return nil
}

// RemoveTarget removes host, if it exists, and its corresponding Target entry in
// the routing table. If it does not exist, returns a non-nil error
func (t *Table) RemoveTarget(host string) error {
	t.l.Lock()
	defer t.l.Unlock()
	_, ok := t.m[host]
	if !ok {
		return fmt.Errorf("host %s did not exist in the routing table", host)
	}
	delete(t.m, host)
	return nil
}

// Replace replaces t's routing table with newTable's.
//
// This function is concurrency safe for t, but not for newTable.
// The caller must ensure that no other goroutine is writing to
// newTable at the time at which they call this function.
func (t *Table) Replace(newTable *Table) {
	t.l.Lock()
	defer t.l.Unlock()
	t.m = newTable.m
	t.Hs = newTable.Hs
}

// SyncHostSwitch syncs t's host switch with its route table
func (t *Table) SyncHostSwitch(router *httprouter.Router, handler http.Handler) {
	t.l.Lock()
	defer t.l.Unlock()
	router = httprouter.New()
	for host := range t.m {
		t.Hs[host] = router
		path := kedahttp.GetUrlFromHostAndPath(host).Path
		router.Handler("GET", path, handler)
	}
}

// Implement the ServeHTTP method on our new type
func (hs HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if a http.Handler is registered for the given host.
	// If yes, use it to handle the request.
	params := httprouter.ParamsFromContext(r.Context())

	fmt.Sprintf("params: %s", params)
	if handler := hs[r.Host+"/kubetest"]; handler != nil {
		handler.ServeHTTP(w, r)
	} else {
		// Handle host names for which no handler is registered
		http.Error(w, "Forbidden", 403) // Or Redirect?
	}
}
