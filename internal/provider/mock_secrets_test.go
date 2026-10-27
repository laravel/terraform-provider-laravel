package provider

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"

	"golang.org/x/crypto/nacl/box"
)

// fakeSecretKeyPairID is the key pair id the fake hands out with its public key.
const fakeSecretKeyPairID = "kp-test-1"

// fakeSecrets models the organization secrets API: a key pair whose public key
// values must be sealed to, a store of secrets whose values are never returned,
// and the environment attachments.
type fakeSecrets struct {
	pub, priv *[32]byte
	// values holds each secret's opened plaintext, so tests can assert the
	// provider sealed the value it was configured with.
	values map[string]string
	// attached is environment ID -> attached secret IDs.
	attached map[string]map[string]bool
}

func newFakeSecrets() *fakeSecrets {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return &fakeSecrets{pub: pub, priv: priv, values: map[string]string{}, attached: map[string]map[string]bool{}}
}

// open reverses the provider's sealing. The real API rejects a value it cannot
// open with a 422, so the fake does too: a plaintext value, or one sealed to
// the wrong key, fails the apply instead of passing silently.
func (s *fakeSecrets) open(keyPairID, sealed string) (string, error) {
	if keyPairID != fakeSecretKeyPairID {
		return "", fmt.Errorf("the selected key pair is invalid")
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", fmt.Errorf("the secret value must be encrypted before it is submitted")
	}
	out, ok := box.OpenAnonymous(nil, raw, s.pub, s.priv)
	if !ok {
		return "", fmt.Errorf("the secret value must be encrypted before it is submitted")
	}
	return string(out), nil
}

func (f *fakeCloud) registerSecretRoutes(mux *http.ServeMux) {
	s := f.secrets

	mux.HandleFunc("GET /secrets/public-key", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, jsonAPIObject(fakeSecretKeyPairID, "organizationKeyPairs", map[string]any{
			"public_key": base64.StdEncoding.EncodeToString(s.pub[:]),
		}))
	})

	mux.HandleFunc("GET /secrets", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		ids := make([]string, 0, len(f.objects["secret"]))
		for id := range f.objects["secret"] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out := []any{}
		for _, id := range ids {
			out = append(out, jsonAPIData(id, "secrets", f.objects["secret"][id]))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": out,
			"meta": map[string]any{"current_page": 1, "last_page": 1, "per_page": 100, "total": len(out)},
		})
	})

	mux.HandleFunc("POST /secrets", func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(r)
		keyPairID, _ := body["key_pair_id"].(string)
		sealed, _ := body["value"].(string)
		plain, err := s.open(keyPairID, sealed)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err)
			return
		}

		f.mu.Lock()
		defer f.mu.Unlock()
		f.seq++
		id := fmt.Sprintf("secret-%d", f.seq)
		attrs := map[string]any{
			"key":        body["key"],
			"notes":      body["notes"],
			"created_at": "2026-01-01T00:00:00Z",
			"updated_at": "2026-01-01T00:00:00Z",
		}
		if f.objects["secret"] == nil {
			f.objects["secret"] = map[string]map[string]any{}
		}
		f.objects["secret"][id] = attrs
		s.values[id] = plain
		writeJSON(w, http.StatusCreated, jsonAPIObject(id, "secrets", attrs))
	})

	mux.HandleFunc("PATCH /secrets/{id}", func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(r)
		id := r.PathValue("id")

		f.mu.Lock()
		defer f.mu.Unlock()
		attrs, ok := f.objects["secret"][id]
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("secret not found"))
			return
		}
		if _, ok := body["key"]; !ok {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("the secret key is required"))
			return
		}
		if sealed, ok := body["value"].(string); ok {
			keyPairID, _ := body["key_pair_id"].(string)
			plain, err := s.open(keyPairID, sealed)
			if err != nil {
				writeError(w, http.StatusUnprocessableEntity, err)
				return
			}
			s.values[id] = plain
		}
		attrs["key"] = body["key"]
		if notes, ok := body["notes"]; ok {
			attrs["notes"] = notes
		}
		attrs["updated_at"] = "2026-01-02T00:00:00Z"
		if f.patches == nil {
			f.patches = map[string][]map[string]any{}
		}
		f.patches["secret"] = append(f.patches["secret"], body)
		writeJSON(w, http.StatusOK, jsonAPIObject(id, "secrets", attrs))
	})

	mux.HandleFunc("DELETE /secrets/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		id := r.PathValue("id")
		delete(f.objects["secret"], id)
		delete(s.values, id)
		for _, set := range s.attached {
			delete(set, id)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
