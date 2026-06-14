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

package oasvalidator

import (
	"github.com/Masterminds/semver/v3"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
)

func validateFormat(value string, format string) *httperror.ValidationError {
	switch format {
	case "json-pointer":
		return goutils.ValidateJSONPointer(value)
	case "uuid":
		return goutils.ValidateUUID(value)
	case "duration":
		return goutils.ValidateDurationRFC3339(value)
	case "ipv4":
		return goutils.ValidateIPV4(value)
	case "ipv6":
		return goutils.ValidateIPV6(value)
	case "hostname":
		return goutils.ValidateHostname(value)
	case "email":
		return goutils.ValidateEmail(value)
	case "date":
		return goutils.ValidateDate(value)
	case "time":
		return goutils.ValidateTime(value)
	case "date-time":
		return goutils.ValidateDateTime(value)
	case "uri":
		return goutils.ValidateAbsoluteURI(value)
	case "url":
		return goutils.ValidateAbsoluteURL(value)
	case "semver":
		return validateSemver(value)
	default:
		return nil
	}
}

func validateSemver(value string) *httperror.ValidationError {
	_, err := semver.NewVersion(value)
	if err != nil {
		return &httperror.ValidationError{
			Detail: "Invalid semver; " + err.Error(),
			Code:   ErrCodeValidationError,
		}
	}

	return nil
}
