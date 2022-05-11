[toc]

# Intro

The main task of communication over the network is to maintain the same state on both sides. But there are some problems:
"messages" from one side can be lost during communication
"answers" from the other side can be lost during communication
For example, a request from the client may be lost due to network lag or a backend restart, or a request may already have been sent to a dead backend and so on. 
Another example would be proxy chain issues occurring while the server sends a response to the client. The client could have a very bad internet connection, 
for instance using GPRS or EDGE, or be using a slow internet connection while roaming.
Another source of problems can be backend exceptions. For example, if your backend uses MySQL and you got a "lock wait timeout" error, then the business logic was not executed.
So how can you be sure that the client and server states are equal? The answer is simple, you should retry all requests from the client until you get a normal, non-5xx response status.
But this may have adverse consequences. If we retry the client request, an action on the server may be processed multiple times. For example, a payment may be requested twice or more, or a deposit from a billing callback could be added to a user’s balance multiple times with each retry. To solve these issues you should use idempotent operations. This is useful in many situations, particularly when dealing with distributed systems where messages can be lost, or delivered out of order.

A real-life example:
```
me: please, add trx to registration request
colleague: what for? 
me: if something is lost, client can get an "EMAIL_USED" error
colleague: it's not a problem, it's normal
colleague: all sites work like this
```

# Definition

Idempotent operations are operations which can be applied multiple times without changing the result. 
For instance, multiplication by one is such an operation, because no matter how many times it is applied, the result is always the same.
```go
package main

import "fmt"

func main() {
	fmt.Println(5 * 1)
	fmt.Println(5 * 1 * 1 * 1 * 1)
}
```

Each idempotent request should have a unique "Trx" - operation id. We will talk about this later.

# When to use idempotency
- client-server communication
- eventbus messages processing
- external API execution
- callbacks from external APIs

In short, in every case when you need to be sure that the other party received your message and processed it.

## client-server communication 

In the context of a client-server setup, idempotent operations are especially important. This is because when a client makes a request to a server, 
it is not always guaranteed that the request will reach the server, that the server will process it or that the client will get a response.
If the client makes the same request multiple times, it is important that the server can handle it without changing the result.

For example, consider a situation where a client makes a request to a server to create a new user. If the server does not receive the request, 
or if the request is lost in transit, the client may resend the request. In this case, there are two key points:
1. It is important that the client should retry the request until they get a non-5xx response
2. It is important that the server can handle the request idempotently, so the creation of the user only happens once

A simple example in Go:

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

In the code above, idempotency is used to ensure doSomething will be called only once for each trx value.

Please note, the code above doesn't ensure that doSomething for the same request won't be executed multiple times. 
In order to achieve that, letter locking or queue-style processing will have to be used.

## eventbus processing

The key idea of implementation of "exactly-once" messaging processing is to implement idempotency on the consumer side. 
// TODO: describe more

## external apis execution
Very often external APIs don't support idempotency at all, but you need a consistent state between your app and the external API. This can also be solved using trxs, 
just choose the correct trx generation based on request. Hash of the request body can be a good solution

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
External system callbacks (e.g. Slack webhooks, etc.) should also be handled idempotently. It's the same case scenario as client-server processing.

# TRX id generation

## Random number
This is the worst way to generate trx IDs because collision probability is too high.

## UUID
The best algorithm for unique trx generation is UUID. It's simple, fast, has implementation capabilities in almost all languages and the probability of collision is very low. 
Also, you can dramatically decrease collision probability just by specifying the context of UUIDs. 
For example, per user or per site. The probability of UUID v4 collision is still 1 in 100 billion even if you generate 1 billion UUIDs.

## Using hash function as trx
You can use body hash to use body as trx, but please note that this may cause some issues. 
For example, if you have a callback from a payment system, then you may receive multiple calls with the same data, but at different times. 
It's not a big problem, but you should think about it before implementation.

# Common mistakes

## Wrong "place"
A common mistake in idempotency implementation is to return something like an ALREADY_PROCESSED response, but not the original response. 
It's not idempotency! For example, in user registration by email you have registration request processing:

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
But if the request was then dropped and resubmitted by the client, the user would receive an "email used" message, and an inconsistent client and server state would be the result. 
You can often see this bad behaviour in various billing APIs where the external server returns something like an ALREADY_PROCESSED error, 
and so we need to implement custom logic which makes processes more complex and harder to understand.
The right way is to use `Trx` as soon as possible, before any processing:
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

So if the client sends a register request with the same trx - it should ALWAYS get the same response. As 5 * 1 * 1 * 1.

## Complex retry strategies
The common mistake with this behaviour is the use of "smart" throttling. 
For example, if a server responds with 503, some developers try to call the server with exponential time. 
This may be implemented in an attempt to solve issues arising from awaiting "confirmation" and password entering tries, 
but this behaviour doesn't solve server problems at all. It simply makes them invisible, hiding the true problem, 
and so you cannot react fast and find out exactly what has happened. 
So, you shouldn't use exponential time for retries, instead use constant time with a delay.

# TRX Expiration
Good practice is to use expiration of trx cache so as not to overuse memory. A simple solution for expiring cache using Go language looks like this:

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

`sleepBetweenExpireCheck` and `ttl` should be fine-tuned based on your real world needs. 
Note that we added context here. For cases when, for example, you need to stop some trx-related worker, you can use separated trx cache for users or for external callbacks. 
If you don’t require a graceful shutdown of the “cleaner” Go routine, then you can skip context here. 

# Databases transactions and idempotency

At first glance it looks to be good practice to make idempotency using transactions and relational databases,
but "duplicate key exception" breaks the main idempotency principles! If you send one request multiple times in parallel and check 
"does row with this trx exist" you can get multiple "no, this row does not exist" responses, based on your database, 
transactions isolation level and so on, so you will need shared locks, for example from Redis. 
It's hard to support and understand sometimes, so you should always ask yourself: 
‘Can I just use one simple app and in-memory trx protector to achieve idempotency, or should I really use something like shared locks?

If you really need a persistent trx protector, you should never forget about idempotency.

For example:
```go
func (d *DepositProcessor) Process(request *Request) (*Response, error) {
	if d.store.IsProcessed(request) {
        return nil, fmt.Errorf("deposit already processed")
    }
	...
}
```

This is not idempotent processing at all. For the same request it will return different responses. 
And so you need to implement custom logic. 
A real-life example: One payment system always returns payment system fee information in a deposit request http action.
But if the deposit has already been created, it just returns an "ALREADY_PROCESSED" response, or something similar. 
So you need to go to another API and ask for this info. If the API is idempotent - just one simple retry is all that will be required.

