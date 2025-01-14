// Copyright 2024 Red Hat, Inc.
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

package github

import (
	"sync"
	"time"

	. "github.com/konflux-ci/mintmaker/internal/pkg/constant"
)

type Cache struct {
	data sync.Map
}

func NewCache() *Cache {
	return &Cache{}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	return c.data.Load(key)
}

func (c *Cache) Set(key string, value interface{}) {
	c.data.Store(key, value)
}

type TokenInfo struct {
	Token     string
	ExpiresAt time.Time
}

type TokenCache struct {
	mu      sync.RWMutex
	entries map[string]TokenInfo
}

func (c *TokenCache) Set(key string, tokenInfo TokenInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = tokenInfo
}

func (c *TokenCache) Get(key string) (TokenInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return TokenInfo{}, false
	}

	// when token is close to expiring, we can't use it
	if time.Until(entry.ExpiresAt) < GhTokenRenewThreshold {
		return TokenInfo{}, false
	}

	return entry, true
}
