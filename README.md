## Chan Assert
#### Asynchronous Channel Assertion Library
![coverage](https://raw.githubusercontent.com/hbomb79/go-chanassert/badges/.badges/main/coverage.svg)

Chan Assert is a declartive library designed to help you when writing (integration) tests which deal with channels/websockets.

This library started out as a helper package for integration testing the "activity stream" websocket for [Thea](http://github.com/hbomb79/Thea), which emits
events to the client to notify it of changes to it's internal state. These messages are parsed from JSON to a struct representation, which can
then be passed to this library to perform asynchronous assertions.

Testing channel responses can be tricky in certain scenarios, especially when integration testing.

##### Example Usage
```golang
expecter := chanassert.
    NewChannelExpecter(c).
    Ignore(
        chanassert.StringEqual("foo"),
        chanassert.StringEqual("bar"),
    ).
    ExpectTimeout(time.Millisecond*500, chanassert.OneOf(
        chanassert.StringEqual("hello"),
        chanassert.StringEqual("world"),
    )).
    Expect(chanassert.OneOf(
        chanassert.StringEqual("hello"),
        chanassert.StringEqual("world"),
    ))
expecter.Listen()

// Your test code here, which will see messages sent on channel 'c'

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
will be failed with the error(s) encountered.

For more examples, I encourage you to check out the testing code.

###### Combinators
TODO

###### Matchers
TODO
