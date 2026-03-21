// Copyright 2026 RelyChan Pte. Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openapiclient

import (
	"errors"

	"github.com/hasura/gotel"
)

var tracer = gotel.NewTracer("openapitools")

var (
	errServerURLRequired = errors.New("server url is required")
	errInvalidServerURL  = errors.New("invalid server URL")
	ErrNoAvailableServer = errors.New(
		"failed to initialize servers. Require at least 1 server has URL",
	)
)
