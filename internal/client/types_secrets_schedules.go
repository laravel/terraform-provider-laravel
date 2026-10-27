package client

import "encoding/json"

// ---------------------------------------------------------------------
// Scheduled Command
// ---------------------------------------------------------------------

type ScheduledCommandData struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes ScheduledCommandAttributes `json:"attributes"`
	// Relationships carries the parent link, which is what makes import
	// work: the import id names this resource only.
	Relationships ScheduledCommandRelationships `json:"relationships"`
}

type ScheduledCommandAttributes struct {
	Name     *string `json:"name"`
	Command  string  `json:"command"`
	Schedule string  `json:"schedule"`
	// CreatedAt is nullable in the spec.
	CreatedAt *string `json:"created_at"`
}

// ScheduledCommandRelationships are the relationships on a scheduled command.
type ScheduledCommandRelationships struct {
	Instance Relationship `json:"instance"`
}

type CreateScheduledCommandRequest struct {
	Name     *string `json:"name,omitempty"`
	Command  string  `json:"command"`
	Schedule string  `json:"schedule"`
}

// UpdateScheduledCommandRequest is the PATCH body. Name is a NullableString
// rather than *string because JSON null clears it, which an omitted
// field cannot express.
type UpdateScheduledCommandRequest struct {
	Name     *NullableString `json:"name,omitempty"`
	Command  *string         `json:"command,omitempty"`
	Schedule *string         `json:"schedule,omitempty"`
}

// ---------------------------------------------------------------------
// Secret
// ---------------------------------------------------------------------

type SecretData struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Attributes SecretAttributes `json:"attributes"`
}

// SecretAttributes never carries the value: the API stores it sealed and
// reports only the key and notes.
type SecretAttributes struct {
	Key       string  `json:"key"`
	Notes     *string `json:"notes"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

type CreateSecretRequest struct {
	KeyPairID string  `json:"key_pair_id"`
	Key       string  `json:"key"`
	Value     string  `json:"value"`
	Notes     *string `json:"notes,omitempty"`
}

// UpdateSecretRequest is the PATCH body. key is required on every update, and
// a new value must arrive sealed together with the key pair it was sealed to.
type UpdateSecretRequest struct {
	KeyPairID *string         `json:"key_pair_id,omitempty"`
	Key       string          `json:"key"`
	Value     *string         `json:"value,omitempty"`
	Notes     *NullableString `json:"notes,omitempty"`
}

// KeyPairData is the organization key pair secret values are sealed to.
type KeyPairData struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Attributes KeyPairAttributes `json:"attributes"`
}

type KeyPairAttributes struct {
	// PublicKey is base64-encoded.
	PublicKey string `json:"public_key"`
}

// ---------------------------------------------------------------------
// Resend
// ---------------------------------------------------------------------

type AttachResendRequest struct {
	KeyStrategy *string `json:"key_strategy,omitempty"`
	FromAddress string  `json:"from_address"`
	FromName    string  `json:"from_name"`
	KeyName     *string `json:"key_name,omitempty"`
	ReuseKeyID  *string `json:"reuse_key_id,omitempty"`
}

type UpdateResendRequest struct {
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
}

// EnvironmentResend is the response-side resend object on an environment;
// null when Resend is not attached.
type EnvironmentResend struct {
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
	KeyName     string `json:"key_name"`
}

// ---------------------------------------------------------------------
// Nullable values
// ---------------------------------------------------------------------

// NullableString is a request value that can be sent as a string or as JSON
// null. A nil *NullableString omits the field; a non-nil one with Null set
// sends null, which is how the API is told to clear a nullable field.
type NullableString struct {
	Value string
	Null  bool
}

func (n NullableString) MarshalJSON() ([]byte, error) {
	if n.Null {
		return []byte("null"), nil
	}
	return json.Marshal(n.Value)
}

func (n *NullableString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*n = NullableString{Null: true}
		return nil
	}
	*n = NullableString{}
	return json.Unmarshal(b, &n.Value)
}

// NullableInt is NullableString for integers.
type NullableInt struct {
	Value int
	Null  bool
}

func (n NullableInt) MarshalJSON() ([]byte, error) {
	if n.Null {
		return []byte("null"), nil
	}
	return json.Marshal(n.Value)
}

func (n *NullableInt) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*n = NullableInt{Null: true}
		return nil
	}
	*n = NullableInt{}
	return json.Unmarshal(b, &n.Value)
}
