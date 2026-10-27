// SPDX-License-Identifier: AGPL-3.0-or-later
// Real-network EAP-AKA and RFC 5998 EAP-only IKE authentication.
package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/iniwex5/vowifi-go/engine/sim"
	"github.com/iniwex5/vowifi-go/engine/swu/eapaka"
	"github.com/iniwex5/vowifi-go/engine/swu/ikev2"
)

var errAKAReady = errors.New("real-network SIM AKA and EAP MAC verified; IMS not registered")

// RFC 4187 section 10.9: AT_AUTS is exactly 16 bytes including type/length.
// Unlike AT_RAND/AT_AUTN, it has no two-byte reserved prefix.
func synchronizationResponse(request eapaka.Packet, auts []byte) ([]byte, error) {
	if request.Type != eapaka.TypeAKA || request.Code != eapaka.CodeRequest || request.Subtype != eapaka.SubtypeChallenge || len(auts) != 14 {
		return nil, errors.New("invalid carrier AKA synchronization response input")
	}
	return (eapaka.Packet{Code: eapaka.CodeResponse, Identifier: request.Identifier, Type: request.Type, Subtype: eapaka.SubtypeSynchronizationFailure, Attributes: []eapaka.Attribute{{Type: eapaka.AttributeAUTS, Data: bytes.Clone(auts)}}}).MarshalBinary()
}

func plainIdentityResponse(request []byte, identity string) ([]byte, error) {
	if len(request) < 5 || request[0] != 1 || request[4] != 1 || int(binary.BigEndian.Uint16(request[2:4])) != len(request) || len(identity) == 0 || len(identity) > 1024 {
		return nil, errors.New("invalid standard EAP Identity request")
	}
	out := make([]byte, 5+len(identity))
	out[0], out[1], out[4] = 2, request[1], 1
	binary.BigEndian.PutUint16(out[2:4], uint16(len(out)))
	copy(out[5:], identity)
	return out, nil
}

func runRealAKAProbe(ctx context.Context, cfg ikev2.FullAuthConfig) (ikev2.FullAuthResult, error) {
	return runRealAKA(ctx, cfg, false)
}

func runRealAKAFull(ctx context.Context, cfg ikev2.FullAuthConfig) (ikev2.FullAuthResult, error) {
	return runRealAKA(ctx, cfg, true)
}

func imsESPProposals(spi []byte) ikev2.SecurityAssociation {
	sa := ikev2.SecurityAssociation{}
	for _, integrity := range []uint16{ikev2.INTEG_HMAC_SHA2_256_128, ikev2.INTEG_HMAC_SHA1_96} {
		sa.Proposals = append(sa.Proposals, ikev2.Proposal{Number: uint8(len(sa.Proposals) + 1), ProtocolID: ikev2.ProtocolESP, SPI: bytes.Clone(spi), Transforms: []ikev2.Transform{
			{Type: ikev2.TransformENCR, ID: ikev2.ENCR_AES_CBC, Attributes: []ikev2.TransformAttribute{ikev2.KeyLengthAttribute(128)}},
			{Type: ikev2.TransformINTEG, ID: integrity},
			{Type: ikev2.TransformESN, ID: 0},
		}})
	}
	return sa
}

func anyIMSTrafficSelectors() ikev2.TrafficSelectors {
	ts := ikev2.IPv4AnyTrafficSelectors()
	ts.Selectors = append(ts.Selectors, ikev2.TrafficSelector{Type: ikev2.TSIPv6AddressRange, EndPort: 65535, StartAddr: net.IP(make([]byte, 16)), EndAddr: net.IP(bytes.Repeat([]byte{255}, 16))})
	return ts
}

// RFC 7296 section 2.15 and RFC 5998: EAP MSK authenticates the complete
// original IKE_SA_INIT exchange plus the ID payload body and peer nonce.
func eapSharedAUTH(init ikev2.InitResult, idBody, msk []byte, initiator bool) ([]byte, error) {
	if len(msk) != 64 || len(idBody) < 5 || len(init.RequestBytes) < 28 || len(init.ResponseBytes) < 28 {
		return nil, errors.New("invalid EAP final authentication inputs")
	}
	realMessage, nonce, skp := init.ResponseBytes, init.NonceI, init.Keys.SKPr
	if initiator {
		realMessage, nonce, skp = init.RequestBytes, init.NonceR, init.Keys.SKPi
	}
	idMAC, err := ikev2.PRF(init.Keys.Profile.PRF, skp, idBody)
	if err != nil {
		return nil, err
	}
	defer clear(idMAC)
	signed := append(bytes.Clone(realMessage), nonce...)
	signed = append(signed, idMAC...)
	defer clear(signed)
	key, err := ikev2.PRF(init.Keys.Profile.PRF, msk, []byte("Key Pad for IKEv2"))
	if err != nil {
		return nil, err
	}
	defer clear(key)
	return ikev2.PRF(init.Keys.Profile.PRF, key, signed)
}

func validateESPSelection(sa, offered ikev2.SecurityAssociation) error {
	if len(sa.Proposals) != 1 || len(sa.Proposals[0].SPI) != 4 || sa.Proposals[0].ProtocolID != ikev2.ProtocolESP {
		return errors.New("invalid selected ESP proposal")
	}
	p := sa.Proposals[0]
	seen := map[uint8]bool{}
	for _, tr := range p.Transforms {
		if seen[tr.Type] || (tr.Type != ikev2.TransformENCR && tr.Type != ikev2.TransformINTEG && tr.Type != ikev2.TransformESN) {
			return errors.New("unexpected selected ESP transform")
		}
		seen[tr.Type] = true
	}
	profile, err := ikev2.ESPKeyProfileFromSA(sa)
	if err != nil {
		return err
	}
	for _, want := range offered.Proposals {
		if want.Number != p.Number {
			continue
		}
		wanted, err := ikev2.ESPKeyProfileFromSA(ikev2.SecurityAssociation{Proposals: []ikev2.Proposal{want}})
		if err == nil && wanted == profile {
			return nil
		}
	}
	return errors.New("ePDG selected an unoffered ESP algorithm")
}

func clearEAPKeys(k eapaka.Keys) {
	for _, p := range [][]byte{k.MK, k.KAut, k.KEncr, k.KRe, k.MSK, k.EMSK, k.CKPrime, k.IKPrime} {
		clear(p)
	}
}

func runRealAKA(ctx context.Context, cfg ikev2.FullAuthConfig, complete bool) (ikev2.FullAuthResult, error) {
	if complete {
		cfg.ChildSA = imsESPProposals(cfg.ChildSPI)
		cfg.TSi, cfg.TSr = anyIMSTrafficSelectors(), anyIMSTrafficSelectors()
		cfg.Configuration = ikev2.SWuConfigurationRequest()
		cfg.Configuration.Attributes = append(cfg.Configuration.Attributes, ikev2.ConfigurationAttribute{Type: 20}, ikev2.ConfigurationAttribute{Type: 21})
	}
	initial, err := ikev2.BuildIKEAuthInitialPayloads(ikev2.AuthConfig{
		Init: cfg.Init, Keys: cfg.Keys, InitiatorID: cfg.InitiatorID, EAPIdentity: cfg.EAPIdentity,
		ChildSA: cfg.ChildSA, ChildSPI: cfg.ChildSPI, TSi: cfg.TSi, TSr: cfg.TSr, Configuration: cfg.Configuration,
	})
	if err != nil {
		return ikev2.FullAuthResult{}, err
	}
	if complete {
		apn, e := ikev2.IdentityPayload(ikev2.PayloadIDr, ikev2.Identity{Type: ikev2.IDFQDN, Data: []byte("ims")})
		if e != nil {
			return ikev2.FullAuthResult{}, e
		}
		only, e := ikev2.NotifyPayload(ikev2.Notify{NotifyType: 16417})
		if e != nil {
			return ikev2.FullAuthResult{}, e
		}
		initial = append([]ikev2.Payload{initial[0], apn, only}, initial[1:]...)
	}
	keys := cfg.Init.Keys
	messageID := uint32(1)
	var lastResponse []byte
	send := func(inner []ikev2.Payload) ([]ikev2.Payload, error) {
		h := ikev2.Header{InitiatorSPI: cfg.Init.InitiatorSPI, ResponderSPI: cfg.Init.ResponderSPI, ExchangeType: ikev2.ExchangeIKE_AUTH, Flags: ikev2.FlagInitiator, MessageID: messageID}
		_, raw, e := ikev2.ProtectMessage(h, keys, true, inner, nil)
		if e != nil {
			return nil, e
		}
		response, e := cfg.Transport.ExchangeIKE(ctx, raw)
		if e != nil {
			return nil, e
		}
		m, p, e := ikev2.UnprotectMessage(response, keys, false)
		if e != nil {
			return nil, e
		}
		if m.Header.InitiatorSPI != h.InitiatorSPI || m.Header.ResponderSPI != h.ResponderSPI || m.Header.MessageID != messageID || m.Header.ExchangeType != ikev2.ExchangeIKE_AUTH || m.Header.Flags&ikev2.FlagResponse == 0 {
			return nil, errors.New("unexpected IKE_AUTH response header")
		}
		messageID++
		lastResponse = bytes.Clone(response)
		return p, nil
	}
	inner, err := send(initial)
	if err != nil {
		return ikev2.FullAuthResult{}, err
	}
	transcript := [][]byte{}
	var responderID []byte
	var eapKeys eapaka.Keys
	defer func() { clearEAPKeys(eapKeys) }()
	authenticated := false
	syncs := 0
	for step := 0; step < 12; step++ {
		var raw []byte
		for _, p := range inner {
			if complete && p.Type == ikev2.PayloadAUTH {
				return ikev2.FullAuthResult{}, errors.New("ePDG declined EAP-only authentication; certificate verification required")
			}
			if p.Type == ikev2.PayloadIDr {
				if _, e := ikev2.ParseIdentity(p.Body); e != nil || (responderID != nil && !bytes.Equal(responderID, p.Body)) {
					return ikev2.FullAuthResult{}, errors.New("invalid or changing ePDG identity")
				}
				responderID = bytes.Clone(p.Body)
			}
			if p.Type == ikev2.PayloadEAP {
				if raw != nil {
					return ikev2.FullAuthResult{}, errors.New("duplicate EAP payload")
				}
				raw = p.Body
			}
			if p.Type == ikev2.PayloadNotify {
				n, e := ikev2.ParseNotify(p.Body)
				if e != nil {
					return ikev2.FullAuthResult{}, e
				}
				if n.NotifyType < 16384 {
					return ikev2.FullAuthResult{}, fmt.Errorf("ePDG authentication notify=%d", n.NotifyType)
				}
			}
		}
		if len(raw) < 4 {
			return ikev2.FullAuthResult{}, errors.New("ePDG did not send an EAP request")
		}
		if int(binary.BigEndian.Uint16(raw[2:4])) != len(raw) {
			return ikev2.FullAuthResult{}, errors.New("invalid EAP length")
		}
		if raw[0] == 4 {
			return ikev2.FullAuthResult{}, errors.New("ePDG returned EAP authentication failure")
		}
		if raw[0] == 3 && complete {
			if !authenticated || len(raw) != 4 || len(responderID) < 5 {
				return ikev2.FullAuthResult{}, errors.New("premature EAP success or missing ePDG identity")
			}
			event("eap_success", map[string]any{"carrier_mac_verified": true})
			auth, e := eapSharedAUTH(cfg.Init, initial[0].Body, eapKeys.MSK, true)
			if e != nil {
				return ikev2.FullAuthResult{}, e
			}
			inner, e = send([]ikev2.Payload{{Type: ikev2.PayloadAUTH, Body: append([]byte{2, 0, 0, 0}, auth...)}})
			clear(auth)
			if e != nil {
				return ikev2.FullAuthResult{}, e
			}
			var receivedAUTH []byte
			for _, p := range inner {
				if p.Type == ikev2.PayloadAUTH {
					if receivedAUTH != nil || len(p.Body) < 5 || p.Body[0] != 2 {
						return ikev2.FullAuthResult{}, errors.New("invalid ePDG final AUTH")
					}
					receivedAUTH = p.Body[4:]
				}
				if p.Type == ikev2.PayloadIDr && !bytes.Equal(p.Body, responderID) {
					return ikev2.FullAuthResult{}, errors.New("ePDG identity changed after EAP")
				}
				if p.Type == ikev2.PayloadNotify {
					n, err := ikev2.ParseNotify(p.Body)
					if err != nil || n.NotifyType < 16384 {
						return ikev2.FullAuthResult{}, fmt.Errorf("ePDG final authentication failed: notify=%d", n.NotifyType)
					}
				}
			}
			expected, e := eapSharedAUTH(cfg.Init, responderID, eapKeys.MSK, false)
			if e != nil || len(receivedAUTH) == 0 || !hmac.Equal(receivedAUTH, expected) {
				clear(expected)
				return ikev2.FullAuthResult{}, errors.New("ePDG final MSK AUTH verification failed")
			}
			clear(expected)
			child, e := ikev2.ParseChildSAResult(cfg.Init, inner, cfg.ChildSPI)
			if e != nil {
				return ikev2.FullAuthResult{}, e
			}
			if e := validateESPSelection(child.SelectedSA, cfg.ChildSA); e != nil {
				return ikev2.FullAuthResult{}, e
			}
			child.EAPSuccess, child.NextMessageID = true, messageID
			event("ike_auth_verified", map[string]any{"eap_only": true, "responder_auth_verified": true, "esp_encryption": child.Keys.Profile.EncryptionID, "esp_integrity": child.Keys.Profile.IntegrityID})
			return ikev2.FullAuthResult{ChildSA: &child, NextMessageID: messageID, FinalResponseBytes: lastResponse, FinalResponseInner: inner}, nil
		}
		if len(raw) >= 5 && raw[4] == 1 {
			response, e := plainIdentityResponse(raw, cfg.EAPIdentity)
			if e != nil {
				return ikev2.FullAuthResult{}, e
			}
			inner, err = send([]ikev2.Payload{ikev2.EAPPayload(response)})
			if err != nil {
				return ikev2.FullAuthResult{}, err
			}
			continue
		}
		request, e := eapaka.ParsePacket(raw)
		if e != nil {
			return ikev2.FullAuthResult{}, e
		}
		if request.Subtype == eapaka.SubtypeIdentity {
			response, e := (eapaka.Packet{Code: eapaka.CodeResponse, Identifier: request.Identifier, Type: request.Type, Subtype: eapaka.SubtypeIdentity, Attributes: []eapaka.Attribute{eapaka.IdentityAttribute(cfg.EAPIdentity)}}).MarshalBinary()
			if e != nil {
				return ikev2.FullAuthResult{}, e
			}
			transcript = append(transcript, bytes.Clone(raw), bytes.Clone(response))
			inner, err = send([]ikev2.Payload{ikev2.EAPPayload(response)})
			if err != nil {
				return ikev2.FullAuthResult{}, err
			}
			continue
		}
		if request.Subtype == eapaka.SubtypeNotification && authenticated {
			response, handled, e := eapaka.BuildAuthenticatedNotificationResponse(request, eapKeys.KAut)
			if e != nil || !handled {
				return ikev2.FullAuthResult{}, errors.New("carrier EAP notification authentication failed")
			}
			raw, e := response.MarshalBinary()
			if e != nil {
				return ikev2.FullAuthResult{}, e
			}
			inner, err = send([]ikev2.Payload{ikev2.EAPPayload(raw)})
			if err != nil {
				return ikev2.FullAuthResult{}, err
			}
			continue
		}
		if request.Subtype != eapaka.SubtypeChallenge {
			return ikev2.FullAuthResult{}, fmt.Errorf("ePDG sent EAP method=%d subtype=%d before SIM AKA", request.Type, request.Subtype)
		}
		rand16, autn16, e := eapaka.ChallengeRANDAndAUTN(request)
		if e != nil {
			return ikev2.FullAuthResult{}, e
		}
		aka, e := cfg.SIM.CalculateAKA(rand16, autn16)
		if errors.Is(e, sim.ErrSyncFailure) && len(aka.AUTS) > 0 && syncs < 1 {
			syncs++
			raw, e := synchronizationResponse(request, aka.AUTS)
			if e != nil {
				return ikev2.FullAuthResult{}, e
			}
			inner, err = send([]ikev2.Payload{ikev2.EAPPayload(raw)})
			if err != nil {
				return ikev2.FullAuthResult{}, err
			}
			continue
		}
		if e != nil {
			return ikev2.FullAuthResult{}, e
		}
		// This validates the carrier's AT_MAC with the real USIM result.
		response, derived, e := eapaka.BuildChallengeResponseWithCheckcode(cfg.EAPIdentity, request, aka, transcript)
		clear(aka.RES)
		clear(aka.CK)
		clear(aka.IK)
		clear(aka.AUTS)
		if e != nil {
			return ikev2.FullAuthResult{}, e
		}
		clearEAPKeys(eapKeys)
		eapKeys = derived
		event("aka_network_mac_verified", map[string]any{"method": request.Type, "imsi_prefix": "23487", "wfc_registered": false})
		if !complete {
			return ikev2.FullAuthResult{}, errAKAReady
		}
		authenticated = true
		wire, e := response.MarshalBinary()
		if e != nil {
			return ikev2.FullAuthResult{}, e
		}
		inner, err = send([]ikev2.Payload{ikev2.EAPPayload(wire)})
		clear(wire)
		if err != nil {
			return ikev2.FullAuthResult{}, err
		}
	}
	return ikev2.FullAuthResult{}, errors.New("EAP authentication step limit reached")
}
