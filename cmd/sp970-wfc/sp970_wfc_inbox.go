// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type inboxMessage struct {
	ID       int64  `json:"id"`
	Address  string `json:"address"`
	Body     string `json:"body"`
	Date     int64  `json:"date"`
	Type     int    `json:"type"`
	Read     bool   `json:"read"`
	Archived bool   `json:"archived"`
}

func archivedSMS() map[string]bool {
	result := map[string]bool{}
	raw, _ := os.ReadFile(adminBase + "/sms-archived.json")
	defer clear(raw)
	_ = json.Unmarshal(raw, &result)
	if result == nil {
		result = map[string]bool{}
	}
	return result
}
func listNativeSMS(ctx context.Context, box string, archived bool) (any, error) {
	where := "type IN (1,2)"
	if box == "sent" {
		where = "type=2"
	} else if box == "inbox" {
		where = "type=1"
	} else if box != "" {
		return nil, errors.New("短信分类无效")
	}
	// HEX preserves multiline content and delimiters; never print this output.
	query := "SELECT _id,hex(address),hex(body),date,type,read FROM sms WHERE " + where + " ORDER BY date DESC,_id DESC LIMIT 500;"
	out, err := exec.CommandContext(ctx, "/system/xbin/sqlite3", "/data/data/com.android.providers.telephony/databases/mmssms.db", query).Output()
	defer clear(out)
	if err != nil {
		return nil, errors.New("短信收件箱暂不可用")
	}
	hidden := archivedSMS()
	messages := make([]inboxMessage, 0)
	unread := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, "|")
		if len(fields) != 6 {
			continue
		}
		id, e := strconv.ParseInt(fields[0], 10, 64)
		if e != nil || id < 1 {
			continue
		}
		address, e := hex.DecodeString(fields[1])
		if e != nil {
			continue
		}
		body, e := hex.DecodeString(fields[2])
		if e != nil {
			clear(address)
			continue
		}
		date, _ := strconv.ParseInt(fields[3], 10, 64)
		kind, _ := strconv.Atoi(fields[4])
		read := fields[5] == "1"
		isArchived := hidden[fields[0]]
		if !read && kind == 1 && !isArchived {
			unread++
		}
		if isArchived == archived {
			messages = append(messages, inboxMessage{ID: id, Address: string(address), Body: string(body), Date: date, Type: kind, Read: read, Archived: isArchived})
		}
		clear(address)
		clear(body)
	}
	return map[string]any{"messages": messages, "unread": unread, "limit": 500}, nil
}
func markNativeSMSRead(ctx context.Context, id int64) error {
	out, err := exec.CommandContext(ctx, "/system/bin/sh", "/system/bin/content", "update", "--uri", "content://sms", "--where", "_id="+strconv.FormatInt(id, 10), "--bind", "read:i:1").CombinedOutput()
	defer clear(out)
	if err != nil || strings.Contains(string(out), "Exception") || strings.Contains(string(out), "Error") {
		return errors.New("无法标记短信")
	}
	return nil
}
func archiveNativeSMS(id int64, archive bool) error {
	values := archivedSMS()
	key := strconv.FormatInt(id, 10)
	if archive {
		values[key] = true
	} else {
		delete(values, key)
	}
	return privateJSON(adminBase+"/sms-archived.json", values)
}

func deleteNativeSMS(ctx context.Context, id int64) error {
	return deleteSMSRecord(ctx, id, adminBase+"/sms-archived.json", func(ctx context.Context, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "/system/bin/sh", append([]string{"/system/bin/content"}, args...)...).CombinedOutput()
	})
}

func deleteSMSRecord(ctx context.Context, id int64, archivePath string, run func(context.Context, ...string) ([]byte, error)) error {
	// This KitKat SMS provider parses message IDs as signed 32-bit integers.
	if id < 1 || id > 1<<31-1 {
		return errors.New("短信标识无效")
	}
	// Use a single-message provider URI so Android also updates conversation
	// metadata. Never construct a broad delete or bypass the provider with SQL.
	out, err := run(ctx, "delete", "--uri", "content://sms/"+strconv.FormatInt(id, 10))
	defer clear(out)
	result := strings.TrimSpace(string(out))
	if err != nil || (result != "" && result != "Deleted 1 rows." && result != "Deleted 0 rows.") {
		return errors.New("无法删除短信")
	}
	// KitKat's content delete returns no success text. Query the same URI
	// afterwards rather than treating an empty command response as failure.
	check, err := run(ctx, "query", "--uri", "content://sms/"+strconv.FormatInt(id, 10), "--projection", "_id")
	defer clear(check)
	if err != nil || strings.TrimSpace(string(check)) != "No result found." {
		return errors.New("无法删除短信")
	}
	// Also remove its archive marker, retaining every other message's state.
	raw, err := os.ReadFile(archivePath)
	if os.IsNotExist(err) {
		return nil
	}
	defer clear(raw)
	var archived map[string]bool
	if err != nil || json.Unmarshal(raw, &archived) != nil {
		return errors.New("短信已删除，但归档标记清理失败")
	}
	key := strconv.FormatInt(id, 10)
	if _, exists := archived[key]; !exists {
		return nil
	}
	delete(archived, key)
	if privateJSON(archivePath, archived) != nil {
		return errors.New("短信已删除，但归档标记清理失败")
	}
	return nil
}
