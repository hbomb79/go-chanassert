package chanassert

// oneOfCombiner becomes satisfied after seeing any value which
// matches only a single matcher, and will not accept any additional values.
type oneOfCombiner[T any] struct {
	matchers  []Matcher[T]
	satisfied bool
}

// OneOf accepts a list of matchers. The returned combiner will become
// satisfied once any of the matchers match message. Once satisfied, this matcher
// will no longer match new messages.
func OneOf[T any](matchers ...Matcher[T]) *oneOfCombiner[T] {
	return &oneOfCombiner[T]{matchers: matchers, satisfied: false}
}

func (oneOf *oneOfCombiner[T]) DoesMatch(t T) bool {
	if oneOf.satisfied {
		return false
	}

	for _, m := range oneOf.matchers {
		if m.DoesMatch(t) {
			oneOf.satisfied = true
			return true
		}
	}

	return false
}

func (oneOf *oneOfCombiner[T]) IsSatisfied() bool {
	return oneOf.satisfied
}
