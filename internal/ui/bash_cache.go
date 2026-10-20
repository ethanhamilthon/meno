package ui

import (
	"container/list"
	"github.com/alecthomas/chroma/v2"
	"sync"
)

const bashCacheBytes = 8 << 20
const bashCacheEntries = 128

type bashTokenEntry struct {
	source string
	tokens []chroma.Token
	bytes  int
}

var bashCache = struct {
	sync.Mutex
	entries map[string]*list.Element
	order   list.List
	bytes   int
}{entries: map[string]*list.Element{}}

func cachedBashTokens(source string) ([]chroma.Token, bool) {
	bashCache.Lock()
	defer bashCache.Unlock()
	e := bashCache.entries[source]
	if e == nil {
		return nil, false
	}
	bashCache.order.MoveToFront(e)
	return e.Value.(bashTokenEntry).tokens, true
}
func cacheBashTokens(source string, tokens []chroma.Token) {
	size := len(source)
	for _, t := range tokens {
		size += len(t.Value) + 32
	}
	if size > bashCacheBytes {
		return
	}
	bashCache.Lock()
	defer bashCache.Unlock()
	if e := bashCache.entries[source]; e != nil {
		bashCache.order.MoveToFront(e)
		return
	}
	bashCache.entries[source] = bashCache.order.PushFront(bashTokenEntry{source, tokens, size})
	bashCache.bytes += size
	for bashCache.order.Len() > bashCacheEntries || bashCache.bytes > bashCacheBytes {
		e := bashCache.order.Back()
		v := e.Value.(bashTokenEntry)
		bashCache.bytes -= v.bytes
		delete(bashCache.entries, v.source)
		bashCache.order.Remove(e)
	}
}
