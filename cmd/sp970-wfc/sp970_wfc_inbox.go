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
