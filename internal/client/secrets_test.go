package client

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/crypto/nacl/box"
)

// The API accepts a secret value only as a libsodium sealed box to the
// organization's public key, base64-encoded. box.OpenAnonymous is the inverse
// of crypto_box_seal, so a value it opens is one the API can open.
func TestSealForPublicKeyRoundTrip(t *testing.T) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const plaintext = "s3cr3t value with spaces & ünïcode"

	sealed, err := sealForPublicKey(base64.StdEncoding.EncodeToString(pub[:]), plaintext)
	if err != nil {
		t.Fatalf("sealing: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		t.Fatalf("sealed value is not standard base64: %v", err)
	}
	opened, ok := box.OpenAnonymous(nil, raw, pub, priv)
	if !ok {
		t.Fatal("the sealed value does not open with the matching key pair")
	}
	if string(opened) != plaintext {
		t.Fatalf("opened %q, want %q", opened, plaintext)
	}

	// Sealing is randomized: the same value never produces the same
	// ciphertext, which is why state cannot compare sealed values.
	again, err := sealForPublicKey(base64.StdEncoding.EncodeToString(pub[:]), plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if again == sealed {
		t.Fatal("two seals of the same value produced the same ciphertext")
	}
}

func TestSealForPublicKeyRejectsBadKeys(t *testing.T) {
	for name, key := range map[string]string{
		"not base64": "not base64!",
		"wrong size": base64.StdEncoding.EncodeToString([]byte("too short")),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := sealForPublicKey(key, "value")
			if err == nil || !strings.Contains(err.Error(), "public key") {
				t.Fatalf("got %v, want an error about the public key", err)
			}
		})
	}
}
