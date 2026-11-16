package main

import (
	"reflect"
	"testing"
)

func TestFoldsRoundTrip(t *testing.T) {
	t.Setenv("VANTAGE_STATE_DIR", t.TempDir()+"/state")
	if got, err := loadFolds(); err != nil || got != nil {
		t.Fatalf("missing file: got %v, %v", got, err)
	}
	want := []string{"api", "web"}
	if err := saveFolds(want); err != nil {
		t.Fatal(err)
	}
	got, err := loadFolds()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, %v; want %v", got, err, want)
	}
	if err := saveFolds(nil); err != nil {
		t.Fatal(err)
	}
	if got, err := loadFolds(); err != nil || got != nil {
		t.Fatalf("after clearing: got %v, %v", got, err)
	}
}
