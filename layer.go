package chanassert

import (
	"fmt"
	"strings"
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
	ok, trace := layer.tryMatch(message)
	trace.Nested = append(trace.Nested, layer.getCombinerStatusTrace())

	return ok, trace
}

func (layer *layer[T]) tryMatch(message T) (bool, TraceMessage) {
	if layer.timeoutElapsed() {
		return false, newInfoTrace(fmt.Sprintf("Message %v (%T) REJECTED, timeout of layer (%s) has been reached", message, message, layer.timeout))
	}

	defer layer.updateSatisfied()

	traces := make([]TraceMessage, 0)
	for idx, combiner := range layer.combiners {
		ok, trace := combiner.TryMatch(message)
		trace.Message = fmt.Sprintf("Combiner #%d: ", idx) + trace.Message

		traces = append(traces, trace)
		if ok {
			return true, newInfoTrace(fmt.Sprintf("Layer #%d matched message against combiner #%d", layer.layerIdx, idx), traces...)
		}
	}

	return false, newInfoTrace(fmt.Sprintf("Layer #%d could not match message against any combiners", layer.layerIdx), traces...)
}

func (layer *layer[T]) IsSatisfied() bool {
	return layer.satisfied
}

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

func (layer *layer[T]) getCombinerStatusTrace() TraceMessage {
	notSatisfied := make(idxList, 0)
	satisfied := make(idxList, 0)

	for idx, combiner := range layer.combiners {
		if combiner.IsSatisfied() {
			satisfied = append(satisfied, idx)
		} else {
			notSatisfied = append(notSatisfied, idx)
		}
	}

	if len(notSatisfied) == len(layer.combiners) {
		return newDebugTrace(
			fmt.Sprintf("No combiners satisfied (0 of %d)", len(layer.combiners)),
		)
	} else if len(satisfied) == len(layer.combiners) {
		return newDebugTrace(
			fmt.Sprintf("ALL combiners satisfied (%d)", len(layer.combiners)),
		)
	} else {
		return newDebugTrace(
			fmt.Sprintf("Combiners %s satisfied, %s NOT satisfied", satisfied, notSatisfied),
		)
	}
}

// idxList is a simple wrapper around a list of ints which
// represent indexes. The main benefit is that we customize
// how this list is converted to a string such that it looks
// like [#0, #1, #2] rather than [0 1 2]
type idxList []int

func (list idxList) String() string {
	str := strings.Builder{}
	str.WriteString("[")
	for idx, n := range list {
		str.WriteString("#")
		str.WriteString(fmt.Sprint(n))

		if idx < len(list)-1 {
			str.WriteString(", ")
		}
	}
	str.WriteString("]")

	return str.String()
}
