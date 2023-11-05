package main

import (
	"sync"
)

type playerCacheSlice struct {
	sync.RWMutex
	data map[string]PlayerRow
}

func NewPlayerCacheSlice() *playerCacheSlice {
	return &playerCacheSlice{
		data: make(map[string]PlayerRow),
	}
}

func (c *playerCacheSlice) Set(playerId string, value PlayerRow) {
	c.Lock()
	defer c.Unlock()
	c.data[playerId] = value
}

func (c *playerCacheSlice) Get(playerId string) (PlayerRow, bool) {
	c.RLock()
	defer c.RUnlock()
	v, ok := c.data[playerId]
	return v, ok
}

func (c *playerCacheSlice) Delete(playerId string) {
	c.Lock()
	defer c.Unlock()
	delete(c.data, playerId)
}

var PlayerCache = NewPlayerCacheSlice()
