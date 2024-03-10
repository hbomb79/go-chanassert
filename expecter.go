// chanassert offers an expressive and dynamic
// way to assert that messages over a channel arrive
// as expected.
package chanassert

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type ExpectCombiner[T any] interface {
	DoesMatch(t T) bool
	IsSatisfied() bool
}

// ExpectLayer is the highest-level of matcher abstraction, and it defines
// a specific matcher which can be selected by the expecter at any one time. Each
// time a new ExpectMatcher is provided to the Expecter, it is placed in it's own
// layer. This is what gives the expecter the "expect this THEN this THEN this" pattern.
type ExpectLayer[T any] interface {
	// Begin is called when the layer is selected. This allows a layer to perform
	// any logic which is best done when the previous layer has completed.
	Begin()

	// DoesMatch is called by the Expecter on a layer when a message is received. This method
	// must return true if the message is valid for the layer, otherwise false. A 'Valid' message is
	// determined by the specific layer implementation (e.g. timeoutLayer)
	DoesMatch(t T) bool

	// IsSatisfied must return true if the layer does not intend to accept any more messages. Typically
	// a layer will NOT return true if it's in an error condition, however this is implementation specific.
	// If a layer advertsises itself as 'Satisfied' following a successful match via 'DoesMatch', then
	// the next layer in the expecter will be selected.
	IsSatisfied() bool

	// Errors must return the errors that this layer has witnessed during it's execution. These
	// errors will be requested when checking if the expecter is satisfied, and the precence of errors
	// indicates that a layer received messages it was not expecting.
	Errors() []error
}

type ExpecterErrors []error

func (errs ExpecterErrors) String() string {
	str := "ExpecterErrors {\n"
	for _, e := range errs {
		str += fmt.Sprintf("  - %s\n", e)
	}
	str += "}\n"

	return str
}

type Expecter[T any] interface {
	Ignore(matchers ...Matcher[T]) Expecter[T]
	Expect(layer ExpectCombiner[T]) Expecter[T]
	ExpectTimeout(timeout time.Duration, layer ExpectCombiner[T]) Expecter[T]
	AssertSatisfied(t *testing.T, timeout time.Duration)
	Satisfied(timeout time.Duration) ExpecterErrors
	Listen()
}

type channelExpecter[T any] struct {
	channel        chan T
	ignoreMatchers []Matcher[T]
	currentLayer   int
	expectLayers   []ExpectLayer[T]
	wg             *sync.WaitGroup
	closeChan      chan struct{}
}

func NewChannelExpecter[T any](channel chan T) *channelExpecter[T] {
	return &channelExpecter[T]{
		channel:        channel,
		currentLayer:   0,
		ignoreMatchers: make([]Matcher[T], 0),
		expectLayers:   make([]ExpectLayer[T], 0),
		closeChan:      make(chan struct{}, 1),
		wg:             &sync.WaitGroup{},
	}
}

func (exp *channelExpecter[T]) Ignore(matchers ...Matcher[T]) Expecter[T] {
	exp.ignoreMatchers = append(exp.ignoreMatchers, matchers...)
	return exp
}

func (exp *channelExpecter[T]) Expect(matcher ExpectCombiner[T]) Expecter[T] {
	layer := &simpleLayer[T]{layerIdx: len(exp.expectLayers), matcher: matcher, errors: make([]error, 0)}
	exp.expectLayers = append(exp.expectLayers, layer)

	return exp
}

func (exp *channelExpecter[T]) ExpectTimeout(timeout time.Duration, matcher ExpectCombiner[T]) Expecter[T] {
	layer := &timeoutLayer[T]{layerIdx: len(exp.expectLayers), timeout: timeout, matcher: matcher, errors: make([]error, 0)}
	exp.expectLayers = append(exp.expectLayers, layer)

	return exp
}

func (exp *channelExpecter[T]) Satisfied(timeout time.Duration) ExpecterErrors {
	outErr := make([]error, 0)
	reportErr := func(err error) {
		outErr = append(outErr, fmt.Errorf("expecter error: %w", err))
	}

	// Wait for the listen loop to close, up to timeout. Otherwise, close it manually
	finished := make(chan struct{}, 1)
	go func() {
		exp.wg.Wait()
		finished <- struct{}{}
	}()

	select {
	case <-time.NewTimer(timeout).C:
		reportErr(errors.New("expecter did not finish within timeout"))
		exp.closeChan <- struct{}{}
		<-finished
	case <-finished:
	}

	for i, l := range exp.expectLayers {
		for _, e := range l.Errors() {
			reportErr(fmt.Errorf("layer #%d error: %w", i, e))
		}
	}

	if exp.currentLayer < len(exp.expectLayers) {
		maybeLayer := exp.expectLayers[exp.currentLayer]
		if maybeLayer != nil {
			if !maybeLayer.IsSatisfied() {
				reportErr(fmt.Errorf("layer #%d is not yet satisfied", exp.currentLayer))
			}
		}
	}

	return outErr
}

func (exp *channelExpecter[T]) AssertSatisfied(t *testing.T, timeout time.Duration) {
	errors := exp.Satisfied(timeout)

	for _, e := range errors {
		t.Errorf("expecter error: %s", e)
	}

	if len(errors) > 0 {
		t.Fatalf("satisified assertion failed: expecter encountered errors (%d)", len(errors))
	}
}

func (exp *channelExpecter[T]) Listen() {
	if len(exp.expectLayers) == 0 {
		panic("no layers specified")
	}

	shouldIgnore := func(message T) bool {
		for _, ignore := range exp.ignoreMatchers {
			if ignore.DoesMatch(message) {
				return true
			}
		}

		return false
	}

	exp.wg.Add(1)
	go func() {
		defer exp.wg.Done()
		exp.currentLayer = 0
		for {
			if exp.currentLayer >= len(exp.expectLayers) {
				// No more layers.
				return
			}

			// TODO: maybe be smarter and not call this every time
			layer := exp.expectLayers[exp.currentLayer]
			layer.Begin()

			select {
			case <-exp.closeChan:
				return
			case message := <-exp.channel:
				if shouldIgnore(message) {
					continue
				}

				if layer.DoesMatch(message) {
					if layer.IsSatisfied() {
						// Layer accepted the message and is satisfied
						exp.currentLayer++
						continue
					}
				}
			}
		}
	}()
}
