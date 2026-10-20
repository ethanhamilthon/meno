package daemon

import (
	"sort"
	"time"
)

func nowNano() int64      { return time.Now().UnixNano() }
func sortInt64(v []int64) { sort.Slice(v, func(i, j int) bool { return v[i] < v[j] }) }
