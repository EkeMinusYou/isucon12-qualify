package main

import (
	"strconv"
	"sync"
)

type dispenceId struct {
	sync.RWMutex
	data int64
}

func NewDispenceId() *dispenceId {
	return &dispenceId{
		data: 0,
	}
}

func (c *dispenceId) Increment() string {
	c.Lock()
	defer c.Unlock()
	c.data++
	return strconv.FormatInt(c.data, 10)
}

var DispenceId = NewDispenceId()
