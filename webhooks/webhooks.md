# Intro
In this part we will talk about a new way to process incoming webhooks.

Usually, webhooks are understood as a mechanism of notification of some event and, sometimes, modifying ("hooking")
behaviour of an external system. For example, pushing code to a repository or a comment being posted to a blog, or a payment processing status change. All 
these usually trigger webhooks. A typical mistake is to think, that they exhibit the "synchronous" behaviour.
If an external system always calls webhooks in a synchronous way, it should "freeze" and wait for an answer. A good example is comment posting: 
if another side goes down, no comments will be posted as webhooks don't reply.

So, how do backend developers usually handle webhooks? They just create another URL handler and process them like any other 
api method. But, there are some issues in this approach:
- development issues. Currently, there are many services (e.g.`webhook.site`, etc.) which can help you
  to get webhooks dumps and use their data to write tests. But then you
  need to test real callbacks and debug their behaviour - you need to deploy your app to test/prod environment. Imagine if you need
  to test thousands of different events in second - all of them from webhook?
- scalability issues. You have one entry point for all data, sent to webhook. But sometimes you need to scale
  their processing. And you have to choose - write your custom balancer based on data or just use multiple servers with some
  distribution (like haproxy + multiple backends definition).
- complex infrastructure. For example - servers traffic routing issues. We need to maintain routes and have some catalogue, 
  where to find each specific webhook processor.

But actually, if we think about webhooks not as notification mechanism, but as simple messaging queue - everything goes pretty simple.
- event occurred on external system
- our system should correctly handle this event
- external system should know, that our system handled this event

# WebHooks as messaging queue

General scheme is pretty simple:
- we have one domain for handle all incoming webhooks
- unified url for all external webhooks is `/{UUID}/smth`, 
  where UUID - is uuid v4, just selected randomly for each time then we need new webhook processing.
  For example, GitHub webhook url will be `/6b7ef954-a581-4526-8aa0-f6ce9f26bebe/github`. 
- and we need two api methods:
  - first for getting all "pending" requests by this UUID
  - second for answering to specific request by its id

We called this scheme as "unifront".

So, then github requested our `/6b7ef954-a581-4526-8aa0-f6ce9f26bebe/github` url, following should happen:
1. unifront generates random request id, using UUID, for example - '85293f16-0d84-48e6-8b60-d5b2fd490e11'
2. stores request in memory in `6b7ef954-a581-4526-8aa0-f6ce9f26bebe` group
3. then our callback processing app checked `6b7ef954-a581-4526-8aa0-f6ce9f26bebe` group for pending webhook in unifront - it receives 
   this new request, process it and send response to our unifront server with group and request id
4. our unifront server removes request from pending and send response

From github side, it's working just as general webhook, but from our side - we can handle this webhook from anywhere,
including developer machine. And simple move processors anywhere.
  
First, we've tried to use RabbitMQ to handle webhooks, but got a lot of latency issues then tried to process 
10-20k Webhooks requests per second.
Second version was on java and works very well for years. Then it starts to use a lot of memory and cpu - we just 
created golang version.

# unifront client example

Webhooks processing in our scheme looks pretty simple:
```go
package example

func main() {
	client := unifront2client.NewUniFront2Client(callbacksServer)
	requests := client.Listen(uuid.MustParse("6b7ef954-a581-4526-8aa0-f6ce9f26bebe"))
	for req := range requests {
		_ = client.Reply(req, &unifront2.CallbackReply{
			Body:   []byte("bad request"),
			Status: 400,
		})
	}
}
```

And you can start this code from anywhere: from your local pc, from prod server, etc. You can scale 
processing just by filtering requests on client side, you can move processing between servers, etc.
Actually, some of our callback clients process hundreds of thousands of requests per second without any issues.

How it works?
Then you call client.Listen, our library starts to ping unifront server api each 

# unifront processing chain
Our server implementation can be found here: {TODO}