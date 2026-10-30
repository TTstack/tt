package command

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestActivationModeHintCannotGrantAuthorityAndExternalReceiptSurvivesSQLiteLoss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authority.json")
	state := AuthorityState{Version: 1, ID: uuid.NewString(), Epoch: 1, Phase: "mutations", Key: uuid.NewString()}
	writeAuthority(t, path, state)
	a, err := OpenAuthority(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.Active("mutations") {
		t.Fatal("a mode hint granted authority without physical evidence")
	}
	state.Phase = "fenced"
	writeAuthority(t, path, state)
	activateJournalFixture(t, a)
	if !a.Active("mutations") || a.Active("gc") {
		t.Fatal("mutation and destruction gates were conflated")
	}
	mutations, err := a.Activation("mutations")
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAuthority(path)
	if err != nil || !reopened.Active("mutations") {
		t.Fatal("receiver lost external phase authority on restart", err)
	}
	replay, err := reopened.CheckActivation(mutations.Request, mutations.Key, "host")
	if err != nil || replay == nil || replay.ID != mutations.ID {
		t.Fatal("receipt replay changed external evidence", err)
	}
	wrong := mutations.Request
	wrong.CatalogDigest = strings.Repeat("f", 64)
	if _, err = reopened.CheckActivation(wrong, mutations.Key, "host"); err == nil {
		t.Fatal("replay rebound the inventory approval")
	}
	if err = os.Remove(path + ".activation-mutations"); err != nil {
		t.Fatal(err)
	}
	if reopened.Active("mutations") {
		t.Fatal("a restored mode hint replaced missing phase authority")
	}
	state.Epoch = 2
	state.Key = uuid.NewString()
	state.Phase = "fenced"
	writeAuthority(t, path, state)
	if receipt, err := reopened.Activation("reads"); err != nil || receipt != nil {
		t.Fatal("old phase proof crossed the external epoch", err)
	}
}

func TestDestructionRequiresIndependentGCActivation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authority.json")
	state := AuthorityState{Version: 1, ID: uuid.NewString(), Epoch: 1, Phase: "fenced", Key: uuid.NewString()}
	writeAuthority(t, path, state)
	j, closeDB := journalFixture(t, ":memory:")
	defer closeDB()
	if err := j.SetAuthority(path); err != nil {
		t.Fatal(err)
	}
	activateJournalFixture(t, j.Authority)
	calls := 0
	handler := j.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(200) }), func(*http.Request) bool { return true }, func(*http.Request, []byte) (string, error) { return "disk-g1", nil })
	envelope := Envelope{Authority: state.ID, Epoch: 1, Subject: "disk-g1", Generation: 1, Token: 1<<EpochSequenceBits | 100, Key: "delete", Fingerprint: strings.Repeat("a", 64)}
	request := func() *httptest.ResponseRecorder {
		raw, _ := json.Marshal(envelope)
		r := httptest.NewRequest("DELETE", "/api/volumes/disk-g1", nil)
		r.Header.Set("X-Compute-Command", string(raw))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if result := request(); result.Code != 409 || calls != 0 {
		t.Fatal("mutation phase admitted destruction", result.Body.String())
	}
	previous, err := j.Authority.Activation("mutations")
	if err != nil {
		t.Fatal(err)
	}
	activation := *previous
	activation.ID = uuid.NewString()
	activation.Key = uuid.NewString()
	activation.Method = "commands_drained"
	activation.Request.Phase = "gc"
	activation.Request.Previous = previous.ID
	if err = j.Authority.CommitActivation(activation); err != nil {
		t.Fatal(err)
	}
	if result := request(); result.Code != 200 || calls != 1 {
		t.Fatal("verified GC gate refused a current command", result.Code, result.Body.String())
	}
	if _, err = j.Conn.ExecContext(context.Background(), "DELETE FROM command_receipts"); err != nil {
		t.Fatal(err)
	}
	if !j.MutationsAllowed() {
		t.Fatal("SQLite receipt loss changed external gate authority")
	}
}
