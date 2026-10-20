package proto

import "encoding/json"

// OpaqueResource represents daemon-owned configuration without client-side schema.
type OpaqueResource struct {
	Data json.RawMessage `json:"data"`
}

// ResourceRequest is a generic typed payload for not-yet-defined resource routes.
type ResourceRequest struct {
	ClientID string          `json:"client_id,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// ResourceResponse carries an opaque daemon-owned resource result.
type ResourceResponse struct {
	Data json.RawMessage `json:"data,omitempty"`
}
