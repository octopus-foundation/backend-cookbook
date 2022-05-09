[toc]

# Intro

The main task of communication over network is to maintain same state on both sides. 
And there are some problems:
- "messages" from one side can be lost during communication
- "answers" from the other side can bet lost during communication

For example - request from client may be lost because of network lag, backend restart, request may be sent to already dead backend and so on.
Another example - proxy chain issues while server send response to client. Client can have very bad internet connection, something like GPRS or EDGE, 
or using slow internet in roaming.

Another source of problems can backend exceptions: for example, your backend uses mysql and got "lock wait timeout", business logic was not
executed.

So how to be sure, that client and server state is equal?
Answer is simple - we should retry all requests from client until we got normal, non-5xx response status.

But there are footprint: if we retry client request - action on server can be processed twice or more.
For example, payment may be requested multiple times or deposit from billing callback can be added to user balance multiple times on each retry.
To solve these issues - you should use idempotent operations. 
This is useful in many situations, especially when dealing with distributed systems where messages can be lost or delivered out of order.

Real-life example:
```
me: please, add trx to registration request
collegue: for what? 
me: if smth lost, client can get "EMAIL_USED" error
collegue: it's not a problem, it's normal
collegue: all sites works like this
```

# Definition

Idempotent operations are operations which can be applied multiple times without changing the result.
For instance -  multiplication by one is such an operation, because no matter how many times it is applied, the result is always the same.

```go
package main

import "fmt"

func main() {
	fmt.Println(5 * 1)
	fmt.Println(5 * 1 * 1 * 1 * 1)
}
```

Each idempotent request should have unique "Trx" - operation id. We will talk about it later.

# When to use idempotency
- client-server communication
- eventbus messages processing
- external apis execution
- callbacks from external apis

In short - in every case when you need to be sure, that other part got your message and process it.

## client-server communication 

In the context of a client-server setup, idempotent operations are especially important.
This is because when a client makes a request to a server, it is not always guaranteed that the request will reach the server,
or that the server will process it or that client will get response. If the client makes the same request multiple times,
it is important that the server can handle it without changing the result.

For example, consider a situation where a client makes a request to a server to create a new user.
If the server does not receive the request, or if the request is lost in transit, the client may resend the request.
In this case, there are two key points:
1. it is important that client should retry request until got some non-5xx response
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

In the code above idempotency is used to ensure `doSomething` will be called only once for each trx value.

Please note, code above doesn't ensure `doSomething` for the same request won't be executed multiple times.
In order to achieve the letter locking or queue-style processing have to be used.

## eventbus processing

The key idea of implementation "exactly-once" messaging processing - is to implement idempotency on 
consumer-side.
// TODO: describe more

## external apis execution
Very often external apis doesn't support idempotency at all, but you need consistent state between your app and external api.
It also can be solved using trx-es, just choose correct trx generation based on request. Hash of request body can be good solution.

Simple example in go:
```go
func callExternalApi() (Response, error) {
    body := []byte("some request body")
    hash := sha1.Sum(body)
    s.cacheLock.RLock()
    resp, exists := s.requestCache[hash]
    s.cacheLock.RUnLock()
	
	// and as usual - process and store result in cache + return
}
```

## callbacks processing
External system callbacks (e.g. Slack webhooks, etc) - should also be handled idempotently. It's same case as client-server processing.

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

# Common mistakes

## Wrong "place"
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

## Complex retry strategies
There is common mistake on this behaviour: using "smart" throttling.
If server respond with, for example, 503, some developers tried to call server with exponential time.
Source of this behaviour, I think, was in "confirmation" and password entering tries. But this behaviour doesn't solve server problems at all at this case,
it's just made them more invisible, and you cannot react faster and found what's exactly happened.
So, you shouldn't use exponential time for retries, use constant time instead with some delay.

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
but "duplicate key exception" breaks main idempotency principles! 
If you send one request multiple times in parallel and will check "is row with this trx exists" you can get multiple "no, this row not exists", based on your database, 
transactions isolation level and so on, so you will need shared locks, for example from redis. 
It's hard to support and understand sometimes - so you always should ask yourself: 
can I just use one simple app and in-memory trx protector to achieve idempotency, or I should really use something like shared locks?

If you really need persistent trx protector, you should never forget about idempotency.

For example:
```go
func (d *DepositProcessor) Process(request *Request) (*Response, error) {
	if d.store.IsProcessed(request) {
        return nil, fmt.Errorf("deposit already processed")
    }
	...
}
```

Is not idempotent processing at all. For same request it will return different responses.
And you need to implement custom logic. Real-life example:
One payment system always return royalty information in deposit request http action.
But if deposit already created, they just return smth like "ALREADY_PROCESSED" response and you need
to go to the another api and ask this info.
If they API will be idempotent - just one simple retry will be required at all. 

