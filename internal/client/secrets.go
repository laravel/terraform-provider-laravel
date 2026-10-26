package client

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

func (c *Client) ListSecrets(ctx context.Context) ([]SecretData, error) {
	return listAll[SecretData](ctx, c, "/secrets")
}

// GetSecret finds one secret by id. There is no GET /secrets/{secret} -- that
// route answers PATCH and DELETE only -- so the organization's list is the only
// way to read one back. It returns a not-found error when the id is absent, so
// callers can treat it like any other single-resource read.
//
// The list is paged, and a secret deleted elsewhere between two page fetches
// shifts the rest by one, so a secret can slip past unseen. A miss is checked
// against a second listing before it is reported: a false not-found would drop
// the secret from state and plan to create it again.
func (c *Client) GetSecret(ctx context.Context, id string) (*SecretData, error) {
	for attempt := 0; attempt < 2; attempt++ {
		secrets, err := c.ListSecrets(ctx)
		if err != nil {
			return nil, err
		}
		for i := range secrets {
			if secrets[i].ID == id {
				return &secrets[i], nil
			}
		}
	}
	return nil, &APIError{StatusCode: 404, Body: fmt.Sprintf("secret %s not found", id)}
}

func (c *Client) CreateSecret(ctx context.Context, req CreateSecretRequest) (*SecretData, error) {
	var doc Document[SecretData]
	if err := c.do(ctx, "POST", "/secrets", req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateSecret(ctx context.Context, id string, req UpdateSecretRequest) (*SecretData, error) {
	var doc Document[SecretData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/secrets/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteSecret(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/secrets/%s", id), nil, nil)
}

// GetSecretsPublicKey returns the organization's active key pair. Its id is the
// key_pair_id a sealed value is submitted with.
func (c *Client) GetSecretsPublicKey(ctx context.Context) (*KeyPairData, error) {
	var doc Document[KeyPairData]
	if err := c.do(ctx, "GET", "/secrets/public-key", nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

// SealSecretValue encrypts a plaintext secret value for the API, which never
// accepts one in the clear: the value is a libsodium sealed box
// (crypto_box_seal) to the organization's public key, base64-encoded. It
// returns the key pair id to submit alongside the sealed value.
func (c *Client) SealSecretValue(ctx context.Context, plaintext string) (keyPairID, sealed string, err error) {
	kp, err := c.GetSecretsPublicKey(ctx)
	if err != nil {
		return "", "", fmt.Errorf("fetching the secrets public key: %w", err)
	}
	sealed, err = sealForPublicKey(kp.Attributes.PublicKey, plaintext)
	if err != nil {
		return "", "", err
	}
	return kp.ID, sealed, nil
}

// sealForPublicKey seals plaintext to a base64-encoded X25519 public key.
// box.SealAnonymous produces the same format as libsodium's crypto_box_seal.
func sealForPublicKey(publicKeyB64, plaintext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return "", fmt.Errorf("decoding the secrets public key: %w", err)
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("the secrets public key is %d bytes, expected 32", len(raw))
	}
	var pk [32]byte
	copy(pk[:], raw)
	out, err := box.SealAnonymous(nil, []byte(plaintext), &pk, rand.Reader)
	if err != nil {
		return "", fmt.Errorf("sealing the secret value: %w", err)
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

// ListEnvironmentSecretIDs returns the ids of the secrets attached to an
// environment. The relationship is reported only when it is asked for.
func (c *Client) ListEnvironmentSecretIDs(ctx context.Context, environmentID string) ([]string, error) {
	var doc struct {
		Data struct {
			Relationships struct {
				Secrets struct {
					Data []ResourceIdentifier `json:"data"`
				} `json:"secrets"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := c.do(ctx, "GET", fmt.Sprintf("/environments/%s?include=secrets", environmentID), nil, &doc); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(doc.Data.Relationships.Secrets.Data))
	for _, s := range doc.Data.Relationships.Secrets.Data {
		ids = append(ids, s.ID)
	}
	return ids, nil
}

// AttachEnvironmentSecrets attaches organization secrets to an environment.
// Secrets that are already attached are ignored by the API.
func (c *Client) AttachEnvironmentSecrets(ctx context.Context, environmentID string, secretIDs []string) error {
	body := struct {
		Secrets []string `json:"secrets"`
	}{Secrets: secretIDs}
	return c.do(ctx, "POST", fmt.Sprintf("/environments/%s/secrets", environmentID), body, nil)
}

// DetachEnvironmentSecret detaches a secret from an environment without
// deleting the secret itself.
func (c *Client) DetachEnvironmentSecret(ctx context.Context, environmentID, secretID string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/environments/%s/secrets/%s", environmentID, secretID), nil, nil)
}
