package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/relaymonkey/relaymesh-edge/internal/session"
)

func TestClientGetMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/me" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Session-Token"); got != "ory_st_test" {
			t.Fatalf("token = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":             "user-1",
			"email":          "ops@example.com",
			"kind":           "user",
			"platform_admin": false,
		})
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "ory_st_test", APIURL: srv.URL})
	me, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.ID != "user-1" {
		t.Fatalf("id = %q", me.ID)
	}
	if me.Email != "ops@example.com" {
		t.Fatalf("email = %q", me.Email)
	}
}

func TestClientListNetworks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/networks" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"aee465d7-8cf7-4672-a603-d556f8db0aa1","slug":"norway","short_id":"ABC12345","name":"Norway","visibility":"public","join_policy":"open","created_at":"2026-01-01T00:00:00Z"}]}`))
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "tok", APIURL: srv.URL})
	list, err := client.ListNetworks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("items = %d", len(list.Items))
	}
	if list.Items[0].ID != "aee465d7-8cf7-4672-a603-d556f8db0aa1" {
		t.Fatalf("id = %q", list.Items[0].ID)
	}
}

func TestClientListMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/networks/net-1/messages" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Fatalf("limit = %q", got)
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"m1","packet_type":1}],"next_cursor":null}`))
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "tok", APIURL: srv.URL})
	list, err := client.ListMessages(context.Background(), "net-1", ListMessagesQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("items = %d", len(list.Items))
	}
	if list.Items[0].StringField("id") != "m1" {
		t.Fatalf("id = %q", list.Items[0].StringField("id"))
	}
}

func TestClientUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "bad", APIURL: srv.URL})
	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveNetworkRefBySlug(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":"aee465d7-8cf7-4672-a603-d556f8db0aa1","slug":"norway","short_id":"ABC12345","name":"Norway","visibility":"public","join_policy":"open","created_at":"2026-01-01T00:00:00Z"}]}`))
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "tok", APIURL: srv.URL})
	n, err := client.ResolveNetworkRef(context.Background(), "norway")
	if err != nil {
		t.Fatal(err)
	}
	if n.Slug != "norway" {
		t.Fatalf("slug = %q", n.Slug)
	}
}

func TestResolveNetworkRefByUUIDAndShortID(t *testing.T) {
	const id = "aee465d7-8cf7-4672-a603-d556f8db0aa1"
	payload := `{"items":[{"id":"` + id + `","slug":"norway","short_id":"ABC12345","name":"Norway","visibility":"public","join_policy":"open","created_at":"2026-01-01T00:00:00Z"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "tok", APIURL: srv.URL})

	for _, ref := range []string{id, "ABC12345"} {
		n, err := client.ResolveNetworkRef(context.Background(), ref)
		if err != nil {
			t.Fatalf("ref %q: %v", ref, err)
		}
		if n.ID != id {
			t.Fatalf("ref %q id = %q", ref, n.ID)
		}
	}
}

func TestResolveNetworkRefByNameCaseInsensitive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":"uuid-1","slug":"alpha","short_id":"AAAA1111","name":"Alpha Network","visibility":"public","join_policy":"open","created_at":"2026-01-01T00:00:00Z"}]}`))
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "tok", APIURL: srv.URL})
	n, err := client.ResolveNetworkRef(context.Background(), "alpha network")
	if err != nil {
		t.Fatal(err)
	}
	if n.ID != "uuid-1" {
		t.Fatalf("id = %q", n.ID)
	}
}

func TestResolveNetworkRefNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "tok", APIURL: srv.URL})
	_, err := client.ResolveNetworkRef(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != `network "missing" not found` {
		t.Fatalf("err = %q", err.Error())
	}
}

func TestResolveNetworkRefAmbiguousName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":"1","slug":"a","short_id":"A1111111","name":"Camp","visibility":"public","join_policy":"open","created_at":"2026-01-01T00:00:00Z"},{"id":"2","slug":"b","short_id":"B2222222","name":"camp","visibility":"public","join_policy":"open","created_at":"2026-01-01T00:00:00Z"}]}`))
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "tok", APIURL: srv.URL})
	_, err := client.ResolveNetworkRef(context.Background(), "camp")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != `network "camp" is ambiguous (2 name matches)` {
		t.Fatalf("err = %q", err.Error())
	}
}

func TestResolveNetworkRefEmpty(t *testing.T) {
	client := New(session.Saved{SessionToken: "tok", APIURL: "http://example.com"})
	_, err := client.ResolveNetworkRef(context.Background(), "  ")
	if err == nil || err.Error() != "network reference is empty" {
		t.Fatalf("err = %v", err)
	}
}

func TestClientNon2xxIncludesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream down"))
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "tok", APIURL: srv.URL})
	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() == "" || err.Error() == "session expired or invalid — run: rmesh auth login" {
		t.Fatalf("err = %q", err.Error())
	}
}

func TestClientListMessageFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/networks/net-1/messages/fields" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("from"); got != "2026-05-01T00:00:00Z" {
			t.Fatalf("from = %q", got)
		}
		_, _ = w.Write([]byte(`{"fields":[{"name":"ingest_ts","type":"string","kind":"core","indexed":true,"allowed_ops":["eq"],"sortable":true,"coverage":1}]}`))
	}))
	defer srv.Close()

	client := New(session.Saved{SessionToken: "tok", APIURL: srv.URL})
	cat, err := client.ListMessageFields(context.Background(), "net-1", "2026-05-01T00:00:00Z", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Fields) != 1 || cat.Fields[0].Name != "ingest_ts" {
		t.Fatalf("fields = %+v", cat.Fields)
	}
}
