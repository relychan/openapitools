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

// Package regexps defines a global cache for regular expressions.
package regexps

import (
	"sync"

	"github.com/dlclark/regexp2"
)

// globalRegexpStore is the process-wide regex cache used by package-level Get.
var globalRegexpStore = &RegexpStore{
	regexps: make(map[string]*regexp2.Regexp),
}

// Get compiles pattern (if not already cached) using the global store and returns the result.
func Get(pattern string) (*regexp2.Regexp, error) {
	return globalRegexpStore.Get(pattern)
}

// RegexpStore is a thread-safe cache that compiles and stores regexp2 patterns on first use.
type RegexpStore struct {
	regexps map[string]*regexp2.Regexp
	mutex   sync.RWMutex
}

// Get returns the compiled regexp for pattern, compiling and caching it on the first call.
func (ps *RegexpStore) Get(pattern string) (*regexp2.Regexp, error) {
	ps.mutex.RLock()

	result, ok := ps.regexps[pattern]

	ps.mutex.RUnlock()

	if ok {
		return result, nil
	}

	regex, err := regexp2.Compile(pattern, regexp2.None)
	if err != nil {
		return nil, err
	}

	ps.mutex.Lock()

	ps.regexps[pattern] = regex

	ps.mutex.Unlock()

	return regex, nil
}
