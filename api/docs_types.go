// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

// APIError is the JSON response returned by the API error handler.
type APIError struct {
	Message string `json:"message"`
}

// EmptyResponse represents an empty JSON object response.
type EmptyResponse struct{}

// SyncCountResponse reports how many rules were synchronized.
type SyncCountResponse struct {
	Count int `json:"count"`
}
