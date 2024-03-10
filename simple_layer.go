package chanassert

import "fmt"

type simpleLayer[T any] struct {
	layerIdx int
	matcher  ExpectCombiner[T]
	errors   []error
}

func (layer *simpleLayer[T]) Begin() {
	// noop
}

func (layer *simpleLayer[T]) DoesMatch(t T) bool {
	if layer.matcher.DoesMatch(t) {
		return true
	}

	layer.errors = append(layer.errors, fmt.Errorf("message %v (%T) did not match layer #%d", t, t, layer.layerIdx))
	return false
}

func (layer *simpleLayer[T]) IsSatisfied() bool {
	return layer.matcher.IsSatisfied()
}
func (layer *simpleLayer[T]) Errors() []error { return layer.errors }
