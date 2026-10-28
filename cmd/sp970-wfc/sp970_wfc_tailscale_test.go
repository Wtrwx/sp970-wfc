// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestTailscaleSSHChangesOnlySSHPreference(t *testing.T) {
	prefs := map[string]any{
		"RunSSH": false, "WantRunning": true, "CorpDNS": false,
		"Hostname": "sp970-b", "AdvertiseRoutes": []string{"192.0.2.0/24"},
	}
	sets := 0
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("Tailscale command must have a deadline")
		}
		if reflect.DeepEqual(args, []string{"debug", "prefs"}) {
			return json.Marshal(prefs)
		}
		if len(args) != 2 || args[0] != "set" || (args[1] != "--ssh=true" && args[1] != "--ssh=false") {
			t.Fatalf("SSH mutation changed unrelated flags: %v", args)
		}
		sets++
		prefs["RunSSH"] = args[1] == "--ssh=true"
		return nil, nil
	}
	for _, enabled := range []bool{true, false, true} {
		if err := changeTailscaleSSH(context.Background(), enabled, run); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{
			"RunSSH": enabled, "WantRunning": true, "CorpDNS": false,
			"Hostname": "sp970-b", "AdvertiseRoutes": []string{"192.0.2.0/24"},
		}
		if !reflect.DeepEqual(prefs, want) {
			t.Fatal("SSH switch changed another preference")
		}
	}
	if sets != 3 {
		t.Fatalf("expected three SSH changes, got %d", sets)
	}
}

func TestTailscaleSSHDoesNotMutateUnknownPreferences(t *testing.T) {
	for _, raw := range []string{`{}`, `{"RunSSH":null}`, `{"RunSSH":"true"}`, `not json`} {
		t.Run(raw, func(t *testing.T) {
			run := func(_ context.Context, args ...string) ([]byte, error) {
				if !reflect.DeepEqual(args, []string{"debug", "prefs"}) {
					t.Fatal("mutated SSH without a readable preference")
				}
				return []byte(raw), nil
			}
			if err := changeTailscaleSSH(context.Background(), true, run); err == nil {
				t.Fatal("accepted unavailable SSH status")
			}
		})
	}
}

func TestTailscaleSSHReportsFailedOrUnappliedChange(t *testing.T) {
	for _, fail := range []bool{false, true} {
		run := func(_ context.Context, args ...string) ([]byte, error) {
			if args[0] == "debug" {
				return []byte(`{"RunSSH":false}`), nil
			}
			if fail {
				return nil, errors.New("private command output")
			}
			return nil, nil
		}
		err := changeTailscaleSSH(context.Background(), true, run)
		if err == nil || strings.Contains(err.Error(), "private command output") {
			t.Fatalf("expected a safe error for failed or unapplied change, got %v", err)
		}
	}
}

func TestTailscaleSSHRequiresManagementLogin(t *testing.T) {
	host := &adminHost{}
	for _, path := range []string{"/tailscale/ssh", "/ssh"} {
		r := httptest.NewRequest("POST", "http://device.local"+path, strings.NewReader(`{"enabled":true}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		host.api(w, r)
		if w.Code != 401 {
			t.Fatalf("%s mutation without login returned %d, expected 401", path, w.Code)
		}
	}
}
