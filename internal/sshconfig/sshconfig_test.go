package sshconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const fixture = `
Host *
  ServerAliveInterval 30

Host web db
  User deploy

Host staging.example.com !bad
  Port 2222

Host *.internal prod-?
  User x

Host web
  User dup

Host -oProxyCommand=evil
  User x
`

func TestHosts(t *testing.T) {
	got, err := Hosts(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	want := []Host{
		{Alias: "web", User: "deploy"},
		{Alias: "db", User: "deploy"},
		{Alias: "staging.example.com", Port: "2222"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestHostsEmpty(t *testing.T) {
	got, err := Hosts(strings.NewReader(""))
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config")
	if err := os.WriteFile(p, []byte("Host alpha\n  HostName 192.0.2.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || !reflect.DeepEqual(got, []Host{{Alias: "alpha", HostName: "192.0.2.1"}}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	// Missing file means no hosts, not an error.
	got, err = Load(filepath.Join(dir, "nope"))
	if err != nil || got != nil {
		t.Fatalf("missing: got %v, %v", got, err)
	}
}

func TestHostsDetails(t *testing.T) {
	cfg := "Host db\n  HostName 192.0.2.20\n  User admin\n  Port 2222\n  ProxyJump bastion\n  IdentityFile /secret/key\n\nHost *\n  User fallback\n"
	got, err := Hosts(strings.NewReader(cfg))
	if err != nil {
		t.Fatal(err)
	}
	want := []Host{{Alias: "db", HostName: "192.0.2.20", User: "admin", Port: "2222", ProxyJump: "bastion"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
