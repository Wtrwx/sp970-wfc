// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/iniwex5/vowifi-go/engine/swu/eapaka"
	"github.com/iniwex5/vowifi-go/engine/swu/ikev2"
	"github.com/iniwex5/vowifi-go/runtimehost/voiceclient"
)

func TestSIMPolicyRejectsRoamingAndMalformedIdentities(t *testing.T) {
	if !validSIMPolicy([]string{"234870000000000", "test-iccid", "2", "test-aid"}) {
		t.Fatal("supported SIM prefix was rejected")
	}
	for _, fields := range [][]string{
		{"204040000000000", "test-iccid", "2", "test-aid"},
		{"23487000000000x", "test-iccid", "2", "test-aid"},
		{"234870000000000", "test-iccid", "3", "test-aid"},
		{"234870000000000"},
	} {
		if validSIMPolicy(fields) {
			t.Fatal("unsupported SIM policy was accepted")
		}
	}
}

func TestIMSFlowHandlesInboundMessageDuringRegistration(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	gotMessage := make(chan bool, 1)
	flow := adoptIMSWireFlow(ctx, client, func(_ context.Context, msg voiceclient.SIPIncomingRequest) (int, string) {
		gotMessage <- msg.Method == "MESSAGE" && string(msg.Body) == "hello"
		return 200, "OK"
	})
	defer flow.Close()
	done := make(chan error, 1)
	go func() {
		_ = server.SetDeadline(time.Now().Add(2 * time.Second))
		r := bufio.NewReader(server)
		request, err := voiceclient.ReadSIPStreamMessage(r)
		if err != nil {
			done <- err
			return
		}
		parsed, err := voiceclient.ParseSIPRequest(request)
		if err != nil {
			done <- err
			return
		}
		_, err = io.WriteString(server, "MESSAGE sip:test@example SIP/2.0\r\nVia: SIP/2.0/TCP pcscf.example;branch=z9hG4bKnet\r\nTo: <sip:test@example>\r\nFrom: <sip:smsc@example>;tag=net\r\nCall-ID: incoming\r\nCSeq: 1 MESSAGE\r\nContent-Length: 5\r\n\r\nhello")
		if err != nil {
			done <- err
			return
		}
		ack, err := voiceclient.ReadSIPStreamMessage(r)
		if err != nil {
			done <- err
			return
		}
		if !bytes.HasPrefix(ack, []byte("SIP/2.0 200")) {
			done <- fmt.Errorf("inbound SMS was not acknowledged")
			return
		}
		wire, err := voiceclient.BuildSIPResponseWire(parsed, 200, "OK", nil, nil)
		if err == nil {
			_, err = server.Write(wire)
		}
		done <- err
	}()
	resp, err := flow.RoundTripRegister(ctx, voiceclient.RegisterMessage{URI: "sip:example", Headers: map[string]string{"Call-ID": "register", "CSeq": "1 REGISTER"}})
	if err != nil || resp.StatusCode != 200 {
		t.Fatal("concurrent inbound message interrupted registration", err)
	}
	if !<-gotMessage {
		t.Fatal("inbound SMS body was changed")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	_, err = sipRequestWire(voiceclient.SIPRequestMessage{Method: "MESSAGE", URI: "sip:example", Headers: map[string]string{"To": "x\r\nInjected: yes"}}, client.LocalAddr())
	if err == nil {
		t.Fatal("accepted injected header")
	}
}

func TestSIMSMSPAndCarrierOwnNumber(t *testing.T) {
	// TS 31.102: alpha identifier plus the fixed 28-byte SMSP tail.
	record := bytes.Repeat([]byte{0xff}, 40)
	record[12] = 0xfd
	copy(record[25:], []byte{7, 0x91, 0x21, 0x43, 0x65, 0x87, 0x09, 0x21})
	number, err := smscFromSMSP(record)
	if err != nil || number != "+123456789012" {
		t.Fatal("SMSP service centre decoded incorrectly", err)
	}
	own, err := carrierOwnNumber([]string{"sip:+123456789012@example;user=phone", "tel:+123456789012"})
	if err != nil || own != number {
		t.Fatal("carrier identity did not yield its confirmed own number", err)
	}
	if _, err = carrierOwnNumber([]string{"tel:+123456789012", "tel:+123456789013"}); err == nil {
		t.Fatal("ambiguous identity allowed a self test")
	}
	if _, err = carrierOwnNumber([]string{"sip:234870000000000@example"}); err == nil {
		t.Fatal("IMSI was treated as a telephone number")
	}
	parcel := "Result: Parcel(\n  0x00000000: 00000000 0000000f 00320031 00340033\n  0x00000010: 00360035 00380037 00300039 00320031\n  0x00000020: 00340033 00000035 )"
	instance, err := equipmentInstanceFromParcel([]byte(parcel))
	if err != nil || !strings.HasSuffix(instance, "-0") {
		t.Fatal("device instance did not use the RFC 7254 zero spare", err)
	}
}

func TestNATTSocketSeparatesIKEAndESPWithoutChangingEndpoint(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	u := &udpIKE{conn: client, marker: true}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	request, _ := (ikev2.Message{Header: ikev2.Header{InitiatorSPI: 1, ResponderSPI: 2, ExchangeType: ikev2.ExchangeIKE_AUTH, Flags: ikev2.FlagInitiator, MessageID: 4}}).MarshalBinary()
	response, _ := (ikev2.Message{Header: ikev2.Header{InitiatorSPI: 1, ResponderSPI: 2, ExchangeType: ikev2.ExchangeIKE_AUTH, Flags: ikev2.FlagResponse, MessageID: 4}}).MarshalBinary()
	stale := bytes.Clone(response)
	binary.BigEndian.PutUint32(stale[20:24], 3)
	esp := []byte{0x13, 0x37, 0, 1, 0, 0, 0, 1, 0x42, 0x43}
	errors := make(chan error, 1)
	go func() {
		_ = server.SetDeadline(time.Now().Add(2 * time.Second))
		got := make([]byte, 4+len(request))
		_, err := io.ReadFull(server, got)
		if err != nil {
			errors <- err
			return
		}
		if !bytes.Equal(got, append([]byte{0, 0, 0, 0}, request...)) {
			errors <- fmt.Errorf("IKE lost non-ESP marker")
			return
		}
		for _, packet := range [][]byte{esp, append([]byte{0, 0, 0, 0}, stale...), append([]byte{0, 0, 0, 0}, response...)} {
			if _, err := server.Write(packet); err != nil {
				errors <- err
				return
			}
		}
		got = make([]byte, len(esp))
		_, err = io.ReadFull(server, got)
		if err == nil && !bytes.Equal(got, esp) {
			err = fmt.Errorf("ESP gained an IKE marker")
		}
		errors <- err
	}()
	got, err := u.ExchangeIKE(ctx, request)
	if err != nil || !bytes.Equal(got, response) {
		t.Fatal("IKE reader consumed ESP or a stale response", err)
	}
	gotESP, err := u.ReadESPPacket(ctx)
	if err != nil || !bytes.Equal(gotESP, esp) {
		t.Fatal("ESP did not survive concurrent IKE exchange", err)
	}
	if err := u.SendESPPacket(ctx, esp); err != nil {
		t.Fatal(err)
	}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
}

func TestFinalAUTHCoversInitNonceAndIdentity(t *testing.T) {
	// Independently calculated with Python hmac.digest, RFC 7296 section 2.15.
	init := ikev2.InitResult{RequestBytes: bytes.Repeat([]byte{'A'}, 32), ResponseBytes: bytes.Repeat([]byte{'B'}, 32), NonceI: bytes.Repeat([]byte{'n'}, 16), NonceR: bytes.Repeat([]byte{'r'}, 16), Keys: ikev2.IKEKeys{Profile: ikev2.KeyMaterialProfile{PRF: crypto.SHA256}, SKPi: bytes.Repeat([]byte{'i'}, 32), SKPr: bytes.Repeat([]byte{'p'}, 32)}}
	msk := make([]byte, 64)
	for i := range msk {
		msk[i] = byte(i)
	}
	idi := append([]byte{3, 0, 0, 0}, []byte("0test@nai.example")...)
	idr := append([]byte{2, 0, 0, 0}, []byte("epdg.example")...)
	for _, tc := range []struct {
		initiator bool
		id        []byte
		want      string
	}{
		{true, idi, "8febb9268c68144e6bbb70a1fb64d8d36b784fe63d3fa826ea624a71cf73f506"},
		{false, idr, "cee476ff71bc9db96aea5d57b631c1e899df1961c0a0d8517a47da28f6c4f339"},
	} {
		got, err := eapSharedAUTH(init, tc.id, msk, tc.initiator)
		if err != nil || hex.EncodeToString(got) != tc.want {
			t.Fatal("final AUTH differs from independent RFC vector")
		}
		changed := bytes.Clone(tc.id)
		changed[len(changed)-1] ^= 1
		other, err := eapSharedAUTH(init, changed, msk, tc.initiator)
		if err != nil || bytes.Equal(got, other) {
			t.Fatal("AUTH did not bind the identity")
		}
	}
	if _, err := eapSharedAUTH(init, idi, msk[:32], true); err == nil {
		t.Fatal("accepted truncated EAP MSK")
	}
}

func TestESPSelectionRejectsUnofferedProfile(t *testing.T) {
	offered := imsESPProposals([]byte{1, 2, 3, 4})
	selected := ikev2.SecurityAssociation{Proposals: []ikev2.Proposal{offered.Proposals[0]}}
	if err := validateESPSelection(selected, offered); err != nil {
		t.Fatal(err)
	}
	raw, _ := selected.MarshalBinary()
	changed, _ := ikev2.ParseSecurityAssociation(raw)
	changed.Proposals[0].Transforms[1].ID = ikev2.INTEG_HMAC_SHA1_96
	if err := validateESPSelection(changed, offered); err == nil {
		t.Fatal("accepted algorithms from a different proposal number")
	}
	changed, _ = ikev2.ParseSecurityAssociation(raw)
	changed.Proposals[0].Transforms = append(changed.Proposals[0].Transforms, changed.Proposals[0].Transforms[0])
	if err := validateESPSelection(changed, offered); err == nil {
		t.Fatal("accepted duplicate ESP transforms")
	}
}

func TestAUTSUsesRFC4187SixteenByteAttribute(t *testing.T) {
	auts := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	request := eapaka.Packet{Code: eapaka.CodeRequest, Identifier: 0x62, Type: eapaka.TypeAKA, Subtype: eapaka.SubtypeChallenge}
	raw, err := synchronizationResponse(request, auts)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 24 || binary.BigEndian.Uint16(raw[2:4]) != 24 || raw[5] != eapaka.SubtypeSynchronizationFailure || raw[8] != eapaka.AttributeAUTS || raw[9] != 4 || !bytes.Equal(raw[10:], auts) {
		t.Fatal("AUTS attribute has an extra reserved prefix or padding")
	}
}

func TestStandardEAPIdentityFromObservedCarrierShape(t *testing.T) {
	request := []byte{1, 0x62, 0, 5, 1}
	identity := "0test@nai.example"
	response, err := plainIdentityResponse(request, identity)
	if err != nil {
		t.Fatal(err)
	}
	if response[0] != 2 || response[1] != 0x62 || response[4] != 1 || int(binary.BigEndian.Uint16(response[2:4])) != 5+len(identity) || string(response[5:]) != identity {
		t.Fatal("invalid RFC 3748 Identity response")
	}
	if _, err := plainIdentityResponse([]byte{1, 0x62, 0, 8, 1}, identity); err == nil {
		t.Fatal("accepted truncated Identity request")
	}
}

func TestCookieRetryPreservesOriginalInit(t *testing.T) {
	msg := ikev2.Message{
		Header: ikev2.Header{InitiatorSPI: 0x1122334455667788, ExchangeType: ikev2.ExchangeIKE_SA_INIT, Flags: ikev2.FlagInitiator},
		Payloads: []ikev2.Payload{
			ikev2.KeyExchangePayload(31, bytes.Repeat([]byte{0x42}, 32)),
			ikev2.NoncePayload(bytes.Repeat([]byte{0x51}, 32)),
		},
	}
	raw, err := msg.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	cookie := bytes.Repeat([]byte{0x71}, 20)
	retried, err := withCookie(raw, cookie)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ikev2.ParseMessage(retried)
	if err != nil {
		t.Fatal(err)
	}
	if m.Header.InitiatorSPI != msg.Header.InitiatorSPI || m.Header.ResponderSPI != 0 || m.Header.MessageID != 0 {
		t.Fatal("retry changed IKE identity")
	}
	if len(m.Payloads) != 3 || m.Payloads[0].Type != ikev2.PayloadNotify {
		t.Fatal("cookie is not first")
	}
	n, err := ikev2.ParseNotify(m.Payloads[0].Body)
	if err != nil || n.NotifyType != 16390 || !bytes.Equal(n.NotificationData, cookie) {
		t.Fatal("cookie changed")
	}
	for i, original := range msg.Payloads {
		got := m.Payloads[i+1]
		if got.Type != original.Type || !bytes.Equal(got.Body, original.Body) {
			t.Fatal("retry changed KE or nonce")
		}
	}
	newCookie := bytes.Repeat([]byte{0x81}, 20)
	updated, err := withCookie(retried, newCookie)
	if err != nil {
		t.Fatal(err)
	}
	m, err = ikev2.ParseMessage(updated)
	if err != nil || len(m.Payloads) != 3 {
		t.Fatal("cookie accumulated on second retry")
	}
	n, err = ikev2.ParseNotify(m.Payloads[0].Body)
	if err != nil || !bytes.Equal(n.NotificationData, newCookie) {
		t.Fatal("replacement cookie incorrect")
	}
}

func TestMODP14AgreementAndInvalidPublicKeys(t *testing.T) {
	p := modp14Prime()
	if p.BitLen() != 2048 || !p.ProbablyPrime(32) {
		t.Fatal("invalid RFC 3526 prime")
	}
	a, b := big.NewInt(3141592653), big.NewInt(2718281828)
	pa, pb := make([]byte, 256), make([]byte, 256)
	new(big.Int).Exp(big.NewInt(2), a, p).FillBytes(pa)
	new(big.Int).Exp(big.NewInt(2), b, p).FillBytes(pb)
	sa, err := modp14Shared(a, pb)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := modp14Shared(b, pa)
	if err != nil {
		t.Fatal(err)
	}
	if len(sa) != 256 || !bytes.Equal(sa, sb) {
		t.Fatal("group-14 secret agreement failed")
	}
	for _, bad := range []*big.Int{big.NewInt(0), big.NewInt(1), new(big.Int).Sub(p, big.NewInt(1)), p} {
		encoded := make([]byte, 256)
		bad.FillBytes(encoded)
		if _, err := modp14Shared(a, encoded); err == nil {
			t.Fatal("accepted invalid DH public key")
		}
	}
}

func TestRejectUnofferedIKESelection(t *testing.T) {
	offer := modp14Proposals()
	for _, p := range offer.Proposals {
		selected := ikev2.SecurityAssociation{Proposals: []ikev2.Proposal{p}}
		if !selectedWasOffered(selected, offer) {
			t.Fatal("rejected offered proposal")
		}
	}
	raw, _ := offer.MarshalBinary()
	changed, err := ikev2.ParseSecurityAssociation(raw)
	if err != nil {
		t.Fatal(err)
	}
	changed.Proposals = changed.Proposals[:1]
	changed.Proposals[0].Transforms[3].ID = 2
	if selectedWasOffered(changed, offer) {
		t.Fatal("accepted unoffered DH group")
	}
}

func TestCookieRejectsInvalidLength(t *testing.T) {
	raw, err := (ikev2.Message{Header: ikev2.Header{InitiatorSPI: 1, ExchangeType: ikev2.ExchangeIKE_SA_INIT, Flags: ikev2.FlagInitiator}}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range [][]byte{nil, make([]byte, 65)} {
		if _, err := withCookie(raw, cookie); err == nil {
			t.Fatal("accepted invalid cookie size")
		}
	}
}

func TestIMSConfigurationRejectsMalformedCarrierAddresses(t *testing.T) {
	valid := ikev2.Configuration{Attributes: []ikev2.ConfigurationAttribute{
		{Type: 1, Value: []byte{10, 20, 30, 40}},
		{Type: 20, Value: []byte{10, 50, 60, 70}},
		{Type: 20, Value: []byte{10, 50, 60, 70}},
	}}
	got, err := parseIMSAddresses(&valid)
	if err != nil || len(got.local) != 1 || len(got.pcscf) != 1 {
		t.Fatalf("valid carrier configuration: %v", err)
	}
	for _, malformed := range []ikev2.ConfigurationAttribute{
		{Type: 20, Value: []byte{10, 1, 2}},
		{Type: 20, Value: []byte{127, 0, 0, 1}},
		{Type: 20, Value: []byte{224, 0, 0, 1}},
		{Type: 8, Value: append(make([]byte, 16), 129)},
	} {
		bad := ikev2.Configuration{Attributes: append(append([]ikev2.ConfigurationAttribute{}, valid.Attributes...), malformed)}
		if _, err := parseIMSAddresses(&bad); err == nil {
			t.Fatalf("accepted malformed carrier attribute %d", malformed.Type)
		}
	}
	if _, err := parseIMSAddresses(nil); err == nil {
		t.Fatal("accepted missing carrier configuration")
	}
}
