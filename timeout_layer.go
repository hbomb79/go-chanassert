package chanassert

import (
	"fmt"
	"time"
)

type timeoutLayer[T any] struct {
	layerIdx  int
	startTime *time.Time
	timeout   time.Duration
	combiner  ExpectCombiner[T]
	errors    []error
}

func (layer *timeoutLayer[T]) Begin() {
	if layer.startTime != nil {
		return
	}

	startTime := time.Now()
	layer.startTime = &startTime
}

func (layer *timeoutLayer[T]) DoesMatch(t T) bool {
	if time.Since(*layer.startTime) > layer.timeout {
		layer.errors = append(layer.errors, fmt.Errorf("layer #%d received message %v (%T), but it's timeout has been exceeded", layer.layerIdx, t, t))
		return false
	}

	if layer.combiner.DoesMatch(t) {
		return true
	}

	layer.errors = append(layer.errors, fmt.Errorf("message %v (%T) did not match layer #%d", t, t, layer.layerIdx))
	return false
}

func (layer *timeoutLayer[T]) IsSatisfied() bool { return layer.combiner.IsSatisfied() }
func (layer *timeoutLayer[T]) Errors() []error   { return layer.errors }
