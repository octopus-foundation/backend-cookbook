[toc]

# Intro

The main task of client-server communication is to maintain same state on client (is user authorised, etc) and server
(e.g. in database or in memory). And there are some problems:
- requests from client can be lost during communication
- responses from server can bet lost during communication

This is key problem.  So how to be sure, that client and server state is equal if some commands from client can lost during communication?
Answer is simple - we should retry all requests from client until we got normal non-500 response status. It's common behaviour to be sure
that server and client has exactly same state. There is common mistake on this behaviour: using "smart" throttling.
If server respond with, for example, 503, some developers tried to call server with exponential time.
Source of this behaviour, I think, was in "confirmation" and password entering tries. But this behaviour doesn't solve server problems at all at this case,
it's just made them more invisible, and you cannot react faster and found what's exactly happened.
So, you shouldn't use exponential time for retries, use constant time instead with some delay.

But if we retry client request - action on server can be processed twice. Or, for example, deposit from billing callback
can be added to user balance multiple times on each retry. So, we need to use idempotent actions on server.

# Definition

Idempotent operations are operations which can be applied multiple times without changing the result.
This is useful in many situations, especially when dealing with distributed systems where messages can be lost or delivered out of order.

For instance -  multiplication by one is such an operation, because no matter how many times it is applied, the result is always the same.

```go
package main

import "fmt"

func main() {
	fmt.Println(5 * 1)
	fmt.Println(5 * 1 * 1 * 1 * 1)
}
```

In the context of a client-server setup, idempotent operations are especially important.
This is because when a client makes a request to a server, it is not always guaranteed that the request will reach the server,
or that the server will process it or that client will get response. If the client makes the same request multiple times,
it is important that the server can handle it without changing the result.

For example, consider a situation where a client makes a request to a server to create a new user.
If the server does not receive the request, or if the request is lost in transit, the client may resend the request.
In this case, there are two key points:
1. it is important that client should retry request until got some non-500 response
2. it is important that the server can handle the request idempotently, so that creating the user only happens once

Simple example in go:

```go
package server

type Request struct {
	Trx Trx    // unique for each request
	// some data
}

func (s *Server) processRequest(clientRequest *Request) (*Response, error) {
	s.cacheLock.RLock()
	resp, exists := s.requestCache[clientRequest.Trx]
	s.cacheLock.RUnLock()
	if exists {
		return resp, nil
	}

	resp, err := s.doSomething(clientRequest) // idempotent
	if err != nil {
		return nil, err
	}

	s.cacheLock.Lock()
	s.requestCache[clientRequest.Trx] = resp
	s.cacheLock.UnLock()

	return resp, nil
}

func (s *Server) doSomething(clientRequest *Request) (*Response, error) {
	// ...
}
```

In the code above idempotency is used to ensure `doSomething` will be called only once for each tax value.

Please note, code above doesn't ensure `doSomething` for the same request won't be executed multiple times.
In order to achieve the letter locking or queue-style processing have to be used.

# TRX id generation

## Random number
Worst case of trx because collision probability is too high.

## UUID
Best algorithm for unique trx generation is UUID. It's simple, fast, has implementation on almost all languages and 
probability of collision is very low. Also, you can dramatically decrease collision probability just specifying the context 
of uuids. For example - per user or per site. Probability of UUID v4 collision is still 1 in 100 billion
if you generate just 1 billion UUIDs.

## Using hash function as trx
You can use body hash to use body as trx, but please note, that you may have some race conditions.
For example, if you have some callback from payment system, then you may receive multiple calls with same data,
but different time. It's not big problem, but you should think about it before implementation.

# Exactly once processing
To achieve "exactly-once" request processing you can use queues or mutexes, depends on context and task. 
In general - there are no difference at all: your goal is executing trx-ed operations one by one, it's not matter how to achieve it.

# Common mistakes

Common mistake in idempotency implementation is to return something like ALREADY_PROCESSED response, but not the original
response. It's not idempotency! For example in user registration by email. You have registration request processing:

```go
package server

type RegRequest struct {
	Trx          Trx
	Email     string
}

func (s *Server) processRegistration(clientRequest *RegRequest) (*Response, error) {
	if s.isEmailUsed(clientRequest.Email) {
		return nil, emailUsedResponse(clientRequest)
	}
	// ...
}
```
But in case then request dropped and resubmitted by client - user got "email used" and inconsistent client and server state as a result.
You can often see this bad behaviour in various billing apis - then external server returns something like `ALREADY_PROCESSED` error,
and we need to implement custom logic which made processing more complex and hard to understand.

The right way is to use `Trx` as soon as possible - before any processing:
```go
package server

type RegRequest struct {
	Trx          Trx
	Email     string
}

func (s *Server) processRegistration(clientRequest *RegRequest) (*Response, error) {
	s.cacheLock.RLock()
	resp, exists := s.requestCache[clientRequest.Trx]
	s.cacheLock.RUnLock()
	if exists {
		return resp, nil
	}
	if s.isEmailUsed(clientRequest.Email) {
		return nil, emailUsedResponse(clientRequest)
	}
	// ...
}
```

So if client send register request with same trx - it should ALWAYS get same response. As 5 * 1 * 1 * 1.

# TRX Expiration
Good practice is to use some expiration of trx cache - just not to overuse memory. Simple solution for expired cache using go language looks like this:

```go
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
```

`sleepBetweenExpireCheck` and `ttl` - should be tuned based on your real needs.
Note that we added context here - for cases then you need to stop some trx-related worker, for example if you use separated 
trx cache for users or for external callbacks. If you use just one trx store which should be stopped then app stops -  you can skip context here. 

# Databases transactions and idempotency

At first glance - it looks like to be a good practice to make idempotency using transactions and relational databases, 
but "duplicate key exception" breaks main idempotency principles! If you send one request multiple times in parallel and 
will check "is row with this trx exists" you can get multiple "no, this row not exists", based on your database, 
transactions isolation level and so one, so you will need shared locks, for example from redis. 
It's hard to support and understand sometimes - so you always should ask yourself: 
can I just use one simple app and in-memory trx protector to achieve idempotency, or I should really use something like shared locks?

# External apis and idempotency

Very often external apis doesn't support idempotency at all, but you need consistent state between your app and external api.
It also can be solved using our trx protector, just choose correct trx generation based on request. In this case - hash of request body can be good solution.
 