// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"testing"
)

func TestAndroidDNSUsesCurrentNetworkBeforePublicFallback(t *testing.T) {
	properties := map[string]string{"net.dns1": "192.0.2.1\n", "net.dns2": "192.0.2.2"}
	read := func(_ context.Context, key string) string { return properties[key] }
	if got := androidDNSAddress(context.Background(), read); got != "192.0.2.1:53" {
		t.Fatalf("did not prefer the current Wi-Fi DNS: %s", got)
	}
	properties["net.dns1"] = "198.51.100.1"
	if got := androidDNSAddress(context.Background(), read); got != "198.51.100.1:53" {
		t.Fatalf("retained DNS from a previous network: %s", got)
	}
}

func TestAndroidDNSSkipsInvalidProperties(t *testing.T) {
	properties := map[string]string{
		"net.dns1": "0.0.0.0", "net.dns2": "224.0.0.1",
		"dhcp.wlan0.dns1": "not an IP; command", "dhcp.wlan0.dns2": "203.0.113.53",
	}
	read := func(_ context.Context, key string) string { return properties[key] }
	if got := androidDNSAddress(context.Background(), read); got != "203.0.113.53:53" {
		t.Fatalf("did not select valid DHCP DNS: %s", got)
	}
	delete(properties, "dhcp.wlan0.dns2")
	if got := androidDNSAddress(context.Background(), read); got != "1.1.1.1:53" {
		t.Fatalf("missing fallback for an unconfigured network: %s", got)
	}
}
