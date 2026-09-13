package external_test

import (
	"testing"

	"stagewise/internal/provider/external"
)

func TestSplit(t *testing.T) {
	got := external.Split(`./my-provider --model "foo bar" --flag 'a b'`)
	want := []string{"./my-provider", "--model", "foo bar", "--flag", "a b"}
	if len(got) != len(want) {
		t.Fatalf("got %q want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}
