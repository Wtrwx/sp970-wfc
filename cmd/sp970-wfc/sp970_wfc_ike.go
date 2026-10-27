// SPDX-License-Identifier: AGPL-3.0-or-later
// Compatibility preflight: RFC 3526 group 14 and RFC 7296 key derivation.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/iniwex5/vowifi-go/engine/swu/ikev2"
)

// Public, standardized 2048-bit prime from RFC 3526 section 3.
const modp14Hex = "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AACAA68FFFFFFFFFFFFFFFF"

func modp14Prime() *big.Int { p, _ := new(big.Int).SetString(modp14Hex, 16); return p }

func modp14Shared(private *big.Int, peer []byte) ([]byte, error) {
	if len(peer) != 256 {
		return nil, errors.New("invalid group-14 public key length")
	}
	p := modp14Prime()
	y := new(big.Int).SetBytes(peer)
	if y.Cmp(big.NewInt(2)) < 0 || y.Cmp(new(big.Int).Sub(p, big.NewInt(2))) > 0 {
		return nil, errors.New("invalid group-14 public key range")
	}
	q := new(big.Int).Rsh(new(big.Int).Sub(p, big.NewInt(1)), 1)
	if new(big.Int).Exp(y, q, p).Cmp(big.NewInt(1)) != 0 {
		return nil, errors.New("invalid group-14 public key subgroup")
	}
	secret := make([]byte, 256)
	new(big.Int).Exp(y, private, p).FillBytes(secret)
	return secret, nil
}

func modp14Proposals() ikev2.SecurityAssociation {
	sa := ikev2.SecurityAssociation{}
	for _, bits := range []uint16{128, 256} {
		for _, sha2 := range []bool{true, false} {
			prf, integ := ikev2.PRF_HMAC_SHA1, ikev2.INTEG_HMAC_SHA1_96
			if sha2 {
				prf, integ = ikev2.PRF_HMAC_SHA2_256, ikev2.INTEG_HMAC_SHA2_256_128
			}
			sa.Proposals = append(sa.Proposals, ikev2.Proposal{
				Number: uint8(len(sa.Proposals) + 1), ProtocolID: ikev2.ProtocolIKE,
				Transforms: []ikev2.Transform{
					{Type: ikev2.TransformENCR, ID: ikev2.ENCR_AES_CBC, Attributes: []ikev2.TransformAttribute{ikev2.KeyLengthAttribute(bits)}},
					{Type: ikev2.TransformPRF, ID: prf},
					{Type: ikev2.TransformINTEG, ID: integ},
					{Type: ikev2.TransformDHRGroup, ID: 14},
				},
			})
		}
	}
	return sa
}

func selectedWasOffered(selected, offered ikev2.SecurityAssociation) bool {
	if len(selected.Proposals) != 1 {
		return false
	}
	s := selected.Proposals[0]
	if s.ProtocolID != ikev2.ProtocolIKE || len(s.SPI) != 0 || len(s.Transforms) != 4 {
		return false
	}
	for _, offer := range offered.Proposals {
		if s.Number != offer.Number {
			continue
		}
		seen := map[uint8]bool{}
		for _, got := range s.Transforms {
			if seen[got.Type] {
				return false
			}
			seen[got.Type] = true
			found := false
			for _, want := range offer.Transforms {
				if got.Type != want.Type || got.ID != want.ID {
					continue
				}
				ga := ikev2.SecurityAssociation{Proposals: []ikev2.Proposal{{Number: 1, ProtocolID: 1, Transforms: []ikev2.Transform{got}}}}
				wa := ikev2.SecurityAssociation{Proposals: []ikev2.Proposal{{Number: 1, ProtocolID: 1, Transforms: []ikev2.Transform{want}}}}
				g, _ := ga.MarshalBinary()
				w, _ := wa.MarshalBinary()
				found = bytes.Equal(g, w)
			}
			if !found {
				return false
			}
		}
		return true
	}
	return false
}

func runMODP14Init(ctx context.Context, cfg ikev2.InitConfig) (ikev2.InitResult, error) {
	random := cfg.Random
	if random == nil {
		random = rand.Reader
	}
	seed := make([]byte, 32)
	if _, err := io.ReadFull(random, seed); err != nil {
		return ikev2.InitResult{}, err
	}
	private := new(big.Int).Add(new(big.Int).SetBytes(seed), big.NewInt(2))
	clear(seed)
	defer private.SetInt64(0)
	public := make([]byte, 256)
	new(big.Int).Exp(big.NewInt(2), private, modp14Prime()).FillBytes(public)
	spiBytes := make([]byte, 8)
	if _, err := io.ReadFull(random, spiBytes); err != nil {
		return ikev2.InitResult{}, err
	}
	spi := binary.BigEndian.Uint64(spiBytes)
	if spi == 0 {
		spi = 1
	}
	nonce := make([]byte, 32)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return ikev2.InitResult{}, err
	}
	offer := modp14Proposals()
	saPayload, err := ikev2.SecurityAssociationPayload(offer)
	if err != nil {
		return ikev2.InitResult{}, err
	}
	payloads := []ikev2.Payload{saPayload, ikev2.KeyExchangePayload(14, public), ikev2.NoncePayload(nonce)}
	for _, n := range []struct {
		kind   uint16
		source bool
	}{{ikev2.NotifyNATDetectionSourceIP, true}, {ikev2.NotifyNATDetectionDestinationIP, false}} {
		ip, port := cfg.RemoteIP, cfg.RemotePort
		if n.source {
			ip, port = cfg.LocalIP, cfg.LocalPort
		}
		if ip != nil {
			p, e := ikev2.NATDetectionNotify(n.kind, spi, 0, ip, port)
			if e != nil {
				return ikev2.InitResult{}, e
			}
			payloads = append(payloads, p)
		}
	}
	request := ikev2.Message{Header: ikev2.Header{InitiatorSPI: spi, ExchangeType: ikev2.ExchangeIKE_SA_INIT, Flags: ikev2.FlagInitiator}, Payloads: payloads}
	raw, err := request.MarshalBinary()
	if err != nil {
		return ikev2.InitResult{}, err
	}
	responseBytes, err := cfg.Transport.ExchangeIKE(ctx, raw)
	if err != nil {
		return ikev2.InitResult{}, err
	}
	response, err := ikev2.ParseMessage(responseBytes)
	if err != nil {
		return ikev2.InitResult{}, err
	}
	h := response.Header
	if h.InitiatorSPI != spi || h.MessageID != 0 || h.ExchangeType != ikev2.ExchangeIKE_SA_INIT || h.Flags&ikev2.FlagResponse == 0 {
		return ikev2.InitResult{}, errors.New("unexpected IKE_SA_INIT header")
	}
	var selected ikev2.SecurityAssociation
	var peer, nonceR []byte
	var notifies []ikev2.Notify
	for _, p := range response.Payloads {
		switch p.Type {
		case ikev2.PayloadSA:
			selected, err = ikev2.ParseSecurityAssociation(p.Body)
		case ikev2.PayloadKE:
			var k ikev2.KeyExchange
			k, err = ikev2.ParseKeyExchange(p.Body)
			if err == nil && k.DHGroup != 14 {
				err = errors.New("ePDG selected an unexpected DH group")
			}
			peer = k.KeyData
		case ikev2.PayloadNonce:
			nonceR = bytes.Clone(p.Body)
		case ikev2.PayloadNotify:
			var n ikev2.Notify
			n, err = ikev2.ParseNotify(p.Body)
			if err == nil {
				notifies = append(notifies, n)
				if n.NotifyType < 16384 {
					return ikev2.InitResult{}, fmt.Errorf("ePDG rejected IKE proposal: notify=%d", n.NotifyType)
				}
			}
		}
		if err != nil {
			return ikev2.InitResult{}, err
		}
	}
	if h.ResponderSPI == 0 || len(nonceR) < 16 || len(nonceR) > 256 || !selectedWasOffered(selected, offer) {
		return ikev2.InitResult{}, errors.New("invalid ePDG key-exchange selection")
	}
	shared, err := modp14Shared(private, peer)
	if err != nil {
		return ikev2.InitResult{}, err
	}
	defer clear(shared)
	profile, err := ikev2.KeyMaterialProfileFromSA(selected)
	if err != nil {
		return ikev2.InitResult{}, err
	}
	skey, err := ikev2.SKEYSEED(profile.PRF, nonce, nonceR, shared)
	if err != nil {
		return ikev2.InitResult{}, err
	}
	material, err := ikev2.DeriveIKESAKeyMaterial(profile.PRF, skey, nonce, nonceR, spi, h.ResponderSPI, profile.RequiredLength())
	if err != nil {
		return ikev2.InitResult{}, err
	}
	keys, err := ikev2.SplitIKEKeys(profile, material)
	if err != nil {
		return ikev2.InitResult{}, err
	}
	nat := false
	for _, n := range notifies {
		ip, port := cfg.RemoteIP, cfg.RemotePort
		if n.NotifyType == ikev2.NotifyNATDetectionDestinationIP {
			ip, port = cfg.LocalIP, cfg.LocalPort
		} else if n.NotifyType != ikev2.NotifyNATDetectionSourceIP {
			continue
		}
		expected, e := ikev2.NATDetectionHash(spi, h.ResponderSPI, ip, port)
		if e == nil && !bytes.Equal(expected, n.NotificationData) {
			nat = true
		}
	}
	event("ike_selection", map[string]any{"dh_group": 14, "encryption": profile.EncryptionID, "key_bits": profile.EncryptionKeyLength * 8, "integrity": profile.IntegrityID, "prf": profile.PRF.String()})
	return ikev2.InitResult{RequestBytes: raw, ResponseBytes: responseBytes, Request: request, Response: response, SelectedSA: selected, InitiatorSPI: spi, ResponderSPI: h.ResponderSPI, NonceI: nonce, NonceR: nonceR, PublicKeyI: public, PublicKeyR: peer, PRF: profile.PRF, SKEYSEED: skey, KeyMaterial: material, Keys: keys, NATDetected: nat}, nil
}
