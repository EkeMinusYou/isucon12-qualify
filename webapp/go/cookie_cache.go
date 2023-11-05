package main

import (
	"sync"

	"github.com/lestrrat-go/jwx/v2/jwt"
)

type cookieCacheSlice struct {
	sync.RWMutex
	data map[string]jwt.Token
}

func NewCookieCacheSlice() *cookieCacheSlice {
	return &cookieCacheSlice{
		data: make(map[string]jwt.Token),
	}
}

func (c *cookieCacheSlice) Set(jwt string, token jwt.Token) {
	c.Lock()
	defer c.Unlock()
	c.data[jwt] = token
}

func (c *cookieCacheSlice) Get(jwt string) (jwt.Token, bool) {
	c.RLock()
	defer c.RUnlock()
	v, ok := c.data[jwt]
	return v, ok
}

var CookieCache = NewCookieCacheSlice()
