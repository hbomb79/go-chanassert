package chanassert

import (
	"fmt"
	"time"
)

type LayerMode int

const (
	and LayerMode = iota
	or
)

type layer[T any] struct {
	combiners []Combiner[T]
	satisfied bool
	errors    []error

	mode     LayerMode
	layerIdx int

	timeout   *time.Duration
	startTime *time.Time
}

func (layer *layer[T]) Begin() {
	if layer.timeout == nil || layer.startTime != nil {
		return
	}

	now := time.Now()
	layer.startTime = &now
}

func (layer *layer[T]) TryMatch(message T) (bool, TraceMessage) {
	if layer.timeoutElapsed() {
		layer.errors = append(layer.errors, fmt.Errorf("message %v (%T) received, but timeout has been exceeded", message, message))
		return false, newEmptyTrace(fmt.Sprintf("Message %v (%T) REJECTED, timeout of layer (%s) has been reached", message, message, layer.timeout))
	}

	defer layer.updateSatisfied()

	traces := make([]TraceMessage, 0)
	for idx, combiner := range layer.combiners {
		ok, trace := combiner.TryMatch(message)
		traces = append(traces, trace)

		if ok {
			return true, newNestedTrace(fmt.Sprintf("Layer #%d matched message against combiner #%d", layer.layerIdx, idx), traces)
		}
	}

	layer.errors = append(layer.errors, fmt.Errorf("message %v (%T) did not match any combiners", message, message))
	return false, newNestedTrace(fmt.Sprintf("Layer #%d could not match message against any combiners", layer.layerIdx), traces)
}

func (layer *layer[T]) IsSatisfied() bool {
	return layer.satisfied
}

func (layer *layer[T]) Errors() []error { return layer.errors }

func (layer *layer[T]) updateSatisfied() {
	//exhaustive:enforce
	switch layer.mode {
	case and:
		// In 'And' mode, the layer becomes satisfied once all
		// combiners are satisfied
		for _, combiner := range layer.combiners {
			if !combiner.IsSatisfied() {
				layer.satisfied = false
				return
			}
		}

		layer.satisfied = true
	case or:
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

func (layer *layer[T]) timeoutElapsed() bool {
	return layer.timeout != nil && time.Until(layer.startTime.Add(*layer.timeout)) <= 0
}
