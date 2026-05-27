package session

import (
	"path/filepath"
	"testing"

	"github.com/relaymonkey/relaymesh-edge/internal/cliconfig"
)

func TestSaveLoadClear(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RMESH_SESSION_FILE", filepath.Join(dir, "session.json"))

	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err != ErrNotLoggedIn {
		t.Fatalf("Load() = %v, want ErrNotLoggedIn", err)
	}

	want := Saved{
		SessionToken: "tok",
		APIURL:       "https://mesh.relaymonkey.com",
		AuthURL:      "https://auth.relaymonkey.com",
		Email:        "ops@example.com",
	}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionToken != want.SessionToken || got.Email != want.Email {
		t.Fatalf("Load() = %+v", got)
	}
	if got.CookieHeader() != "ory_kratos_session=tok" {
		t.Fatalf("CookieHeader() = %q", got.CookieHeader())
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionPathDefault(t *testing.T) {
	t.Setenv("RMESH_SESSION_FILE", "")
	path, err := cliconfig.SessionPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "session.json" {
		t.Fatalf("basename = %q", filepath.Base(path))
	}
	if filepath.Base(filepath.Dir(path)) != ".rmesh" {
		t.Fatalf("dir = %q", filepath.Dir(path))
	}
}
