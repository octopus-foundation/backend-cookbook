package trxstore

import (
	"context"
	"github.com/google/uuid"
	"sync"
	"time"
)

const sleepBetweenExpireCheck = 100 * time.Millisecond

type BytesTrxStore struct {
	cache       map[uuid.UUID][]byte
	cacheLock   sync.RWMutex
	cacheExpire map[uuid.UUID]time.Time
	ttl         time.Duration
}

func NewBytesTRXStoreWithContext(ctx context.Context, ttl time.Duration) *BytesTrxStore {
	res := &BytesTrxStore{
		cache:       map[uuid.UUID][]byte{},
		cacheLock:   sync.RWMutex{},
		cacheExpire: map[uuid.UUID]time.Time{},
		ttl:         ttl,
	}

	go res.watchExpire(ctx)

	return res
}

func NewBytesTRXStore(ttl time.Duration) *BytesTrxStore {
	return NewBytesTRXStoreWithContext(nil, ttl)
}

func (p *BytesTrxStore) Check(trx uuid.UUID) []byte {
	p.cacheLock.RLock()
	res := p.cache[trx]
	p.cacheLock.RUnlock()

	return res
}

func (p *BytesTrxStore) Store(trx uuid.UUID, result []byte) {
	p.cacheLock.Lock()
	p.cache[trx] = result
	p.cacheExpire[trx] = time.Now()
	p.cacheLock.Unlock()
}

func (p *BytesTrxStore) watchExpire(ctx context.Context) {
	timer := time.NewTicker(sleepBetweenExpireCheck)

	if ctx == nil {
		for range timer.C {
			p.cleanupExpired()
		}
	} else {
		for {
			select {
			case <-timer.C:
				p.cleanupExpired()
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
	}
}

func (p *BytesTrxStore) cleanupExpired() {
	var entriesForRemove = map[uuid.UUID]struct{}{}
	p.cacheLock.RLock()
	for id, createdAt := range p.cacheExpire {
		if time.Since(createdAt) > p.ttl {
			entriesForRemove[id] = struct{}{}
		}
	}
	p.cacheLock.RUnlock()

	if len(entriesForRemove) > 0 {
		p.cacheLock.Lock()
		for entryId := range entriesForRemove {
			delete(p.cache, entryId)
			delete(p.cacheExpire, entryId)
		}
		p.cacheLock.Unlock()
	}
}
