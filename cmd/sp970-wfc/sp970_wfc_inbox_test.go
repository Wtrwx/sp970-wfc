// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSMSDeleteScopesProviderAndPreservesOtherArchiveMarkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archived.json")
	if err := privateJSON(path, map[string]bool{"42": true, "43": true}); err != nil {
		t.Fatal(err)
	}
	var called [][]string
	err := deleteSMSRecord(context.Background(), 42, path, func(_ context.Context, args ...string) ([]byte, error) {
		called = append(called, args)
		if args[0] == "delete" {
			return nil, nil // Android 4.4 emits no success receipt.
		}
		return []byte("No result found.\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(called, [][]string{
		{"delete", "--uri", "content://sms/42"},
		{"query", "--uri", "content://sms/42", "--projection", "_id"},
	}) {
		t.Fatal("deletion did not target exactly one provider record")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var archived map[string]bool
	if json.Unmarshal(raw, &archived) != nil || !reflect.DeepEqual(archived, map[string]bool{"43": true}) {
		t.Fatal("deleting one message changed another message's archive state")
	}
}

func TestSMSDeleteRejectsInvalidIDWithoutInvokingProvider(t *testing.T) {
	for _, id := range []int64{0, -1, 1 << 31, 1<<63 - 1} {
		if err := deleteSMSRecord(context.Background(), id, "unused", func(context.Context, ...string) ([]byte, error) {
			t.Fatal("invalid ID reached the SMS provider")
			return nil, nil
		}); err == nil {
			t.Fatal("invalid ID accepted")
		}
	}
}

func TestSMSDeleteRequiresProviderConfirmation(t *testing.T) {
	for _, response := range []string{"Error: provider unavailable", "Deleted 2 rows.", "Exception: denied"} {
		t.Run(response, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "archived.json")
			original := []byte(`{"42":true,"43":true}`)
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			err := deleteSMSRecord(context.Background(), 42, path, func(context.Context, ...string) ([]byte, error) {
				return []byte(response), nil
			})
			if err == nil {
				t.Fatal("unconfirmed deletion reported as success")
			}
			raw, _ := os.ReadFile(path)
			if string(raw) != string(original) {
				t.Fatal("failed deletion changed archive state")
			}
		})
	}
	if err := deleteSMSRecord(context.Background(), 42, "unused", func(context.Context, ...string) ([]byte, error) {
		return []byte("Deleted 1 rows."), errors.New("command failed")
	}); err == nil {
		t.Fatal("failed provider command reported as success")
	}
}

func TestSMSDeleteIsIdempotentWhenMessageAlreadyRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archived.json")
	if err := deleteSMSRecord(context.Background(), 42, path, func(_ context.Context, args ...string) ([]byte, error) {
		if args[0] == "delete" {
			return []byte("Deleted 0 rows."), nil
		}
		return []byte("No result found."), nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSMSDeleteDoesNotReportSuccessWhenRecordRemains(t *testing.T) {
	for _, response := range []string{"Row: 0 _id=42", "", "Exception: denied"} {
		path := filepath.Join(t.TempDir(), "archived.json")
		if err := privateJSON(path, map[string]bool{"42": true}); err != nil {
			t.Fatal(err)
		}
		err := deleteSMSRecord(context.Background(), 42, path, func(_ context.Context, args ...string) ([]byte, error) {
			if args[0] == "delete" {
				return nil, nil
			}
			return []byte(response), nil
		})
		if err == nil {
			t.Fatal("unconfirmed deletion reported as success")
		}
		raw, _ := os.ReadFile(path)
		var archived map[string]bool
		if json.Unmarshal(raw, &archived) != nil || !archived["42"] {
			t.Fatal("unconfirmed deletion cleared the archive marker")
		}
	}
}
