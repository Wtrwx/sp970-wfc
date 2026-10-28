// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"net"
	"os/exec"
	"strings"
	"time"
)

// The statically linked Linux runtime cannot read Android's DNS properties
// through libc. Query them at lookup time so changing Wi-Fi also changes DNS.
func deviceResolver() *net.Resolver {
	return &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		server := androidDNSAddress(ctx, func(ctx context.Context, property string) string {
			pctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			out, _ := exec.CommandContext(pctx, "/system/bin/getprop", property).Output()
			return string(out)
		})
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, server)
	}}
}

func androidDNSAddress(ctx context.Context, readProperty func(context.Context, string) string) string {
	for _, property := range []string{"net.dns1", "net.dns2", "dhcp.wlan0.dns1", "dhcp.wlan0.dns2"} {
		ip := net.ParseIP(strings.TrimSpace(readProperty(ctx, property)))
		if ip != nil && ip.To4() != nil && !ip.IsUnspecified() && !ip.IsMulticast() {
			return net.JoinHostPort(ip.To4().String(), "53")
		}
	}
	// Preserve the previous fallback when Android has no usable DNS property.
	return "1.1.1.1:53"
}
