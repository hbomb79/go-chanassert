package chanassert

import (
	"fmt"
	"time"
)

type LayerMode int

const (
	And LayerMode = iota
	Or
)

type layer[T any] struct {
	combiners []Combiner[T]
	satisfied bool
	errors    []error

	mode     LayerMode
	layerIdx int

	timeout *time.Time
}

func (layer *layer[T]) updateSatisfied() {
	//exhaustive:enforce
	switch layer.mode {
	case And:
		// In 'And' mode, the layer becomes satisfied once all
		// combiners are satisfied
		for _, combiner := range layer.combiners {
			if !combiner.IsSatisfied() {
				layer.satisfied = false
				return
			}
		}

		layer.satisfied = true
	case Or:
		// In 'Or' mode, the layer becomes satisfied any combiner
		// is satisfied
		for _, combiner := range layer.combiners {
			if combiner.IsSatisfied() {
				layer.satisfied = true
				return
			}
		}

		layer.satisfied = false
	}
}

func (layer *layer[T]) DoesMatch(message T) bool {
	if layer.timeout != nil {
		if time.Until(*layer.timeout) <= 0 {
			layer.errors = append(layer.errors, fmt.Errorf("message %v (%T) received, but timeout (%s) has been exceeded", message, message, layer.timeout))
			return false
		}
	}

	defer layer.updateSatisfied()
	for _, combiner := range layer.combiners {
		if combiner.DoesMatch(message) {
			return true
		}
	}

	layer.errors = append(layer.errors, fmt.Errorf("message %v (%T) did not match any combiners", message, message))
	return false
}

func (layer *layer[T]) IsSatisfied() bool {
	return layer.satisfied
}

func (layer *layer[T]) Errors() []error { return layer.errors }
