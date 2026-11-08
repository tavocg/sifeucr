package cached

import (
	"crypto/rand"
	"hash/maphash"
	"strconv"
	"sync"
)

type cacheShard struct {
	mu         sync.RWMutex
	generation uint64
}

// Fixed shards bound coordination memory regardless of how many keys are read.
// A hash collision only causes additional cache misses after an invalidation.
type cacheState struct {
	prefix string
	seed   maphash.Seed
	shards [64]cacheShard
}

func newCacheState() *cacheState {
	return &cacheState{prefix: rand.Text() + ":", seed: maphash.MakeSeed()}
}

func (s *cacheState) shard(key string) *cacheShard {
	return &s.shards[maphash.String(s.seed, key)%uint64(len(s.shards))]
}

func (s *cacheState) key(key string, generation uint64) string {
	return s.prefix + strconv.FormatUint(generation, 10) + ":" + key
}
