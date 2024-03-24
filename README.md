## Chan Assert
#### Asynchronous Channel Assertion Library
![coverage](https://raw.githubusercontent.com/hbomb79/go-chanassert/badges/.badges/main/coverage.svg)

Chan Assert is a declartive library designed to help you when writing (integration) tests which deal with channels/websockets.

Testing channel responses can be tricky in certain scenarios, especially when integration testing. This library started out as a
helper package for integration testing the "activity stream" websocket for [Thea](http://github.com/hbomb79/Thea), with the intent
of allowing tests to make declarative assertions about what messages come through a specific channel.

##### Simple Usage
```golang
expecter := chanassert.
    NewChannelExpecter(c).
    Ignore(
        chanassert.MatchEqual("foo"),
        chanassert.MatchEqual("bar"),
    ).
    ExpectTimeout(time.Millisecond*500, chanassert.AllOf(
        chanassert.StringEqual("hello"),
        chanassert.StringEqual("world"),
    )).
    Expect(chanassert.OneOf(
        chanassert.MatchStringContains("h"),
        chanassert.StringEqual("world"),
    ))
expecter.Listen()

// Your test code here, which will cause messages to be sent on channel 'c'

// Assert your expectations, with a timeout in the event your service under-test
// buffers/debounces the messages
expecter.AssertSatisfied(t, time.Second)
```

The `Expect*` blocks form an ordering, meaning that the next block of 'expect matchers' will only
be used once the previous ones are completed. 'Ignore' is an exception, as it defines messages which will be discarded
globally for the entire expecter. This is useful if your test is only interested in certain messages over an event bus
that may be used by multiple different parts of your system

Then, at any point (but typically at the end), the testing code can use `AssertSatisfied(testing.T, time.Duration)`
which will wait for the timeout specified for the expectations to be satisfied. If not satisfied in time, the test
will be failed with the error(s) encountered. In addition to asserting all layers become satisfied, the 'logs'
of the matcher will also be output which allows you to debug the behaviour of the chanassert expecter:

For example, if your expecter saw messages "foo", "hello", "world", "wor", "world", the logs would be:

```
[ChanAssert] Expecter: Starting up... selecting layer #0
[ChanAssert] Expecter: Ignoring message `foo` (ignore matcher #0 matched this message)
[ChanAssert] Layer #0: Accepted message 'hello'
    - Combiner #0 matched this message
        * Matcher #0 matched this message
        * Combiner is not yet satisfied (in ALL mode), still needs to see 1 match on matchers: [#1]
    - 0 out of 1 combiners satisfied
    - 490ms remaining on timeout
[ChanAssert] Layer #0: Accepted message 'world'
    - Combiner #0: Matched this message
        * Matcher #0 rejected this message
            + Reason: message 'world' did not equal match target 'hello'
        * Matcher #1 matched this message
        * Combiner is satisifed
    - All combiners satisified
    - 460ms remaining on timeout
[ChanAssert] Layer #0 now satisifed, selecting layer #1
[ChanAssert] Layer #1 could not match message 'wor'
    - Combiner #0 could not match
        * Matcher #0 failed: could not find substr 'h' inside message 'wor'
        * Matcher #1 failed: message 'wor' did not equal 'world'
    - 0 out of 1 combiners satisfied
[ChanAssert] Layer #1 matched message 'world'
    - Combiner #0 matched this message
        * Matcher #1 matched this message
    - All combiners satisfied
[ChanAssert] Layer #1 now satisfied
[ChanAssert] No more layers remaining. Expecter satisfied
```

For more examples, I encourage you to check out the testing code.

###### Combinators
TODO

###### Matchers
TODO
