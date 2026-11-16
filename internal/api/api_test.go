package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every operation's description opens with its guard, and each guard is the one its
// operation's middleware enforces: the sign-in operations public, reading open to a share
// link's visitor, and everything else the owner's.
func TestEveryOperationStatesItsGuard(t *testing.T) {
	spec, err := Spec()
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Description string `json:"description"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	guards := map[string]string{}
	for _, ops := range doc.Paths {
		for _, o := range ops {
			for _, g := range []string{guardPublic, guardAuth, guardAdmin} {
				if strings.HasPrefix(o.Description, g) {
					guards[o.OperationID] = g
				}
			}
			if guards[o.OperationID] == "" {
				t.Errorf("%s states no guard: %q", o.OperationID, o.Description)
			}
		}
	}
	for id, want := range map[string]string{
		"get-me": guardPublic, "begin-passkey-sign-in": guardPublic, "sign-out": guardPublic,
		"list-books": guardAuth, "set-log-level": guardAdmin, "stream-changes": guardAdmin,
	} {
		if guards[id] != want {
			t.Errorf("%s: %q, want %q", id, guards[id], want)
		}
	}
}
