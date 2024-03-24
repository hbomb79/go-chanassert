package chanassert

import (
	"fmt"
	"io"
	"strings"
)

type TraceMessage struct {
	Message string
	Nested  []TraceMessage
}

func newEmptyTrace(message string) TraceMessage {
	return TraceMessage{Message: message}
}

func newNestedTrace(message string, nested []TraceMessage) TraceMessage {
	return TraceMessage{
		Message: message,
		Nested:  nested,
	}
}

func (msg TraceMessage) printTrace(writer io.Writer, nestLevel int) {
	fmt.Fprint(writer, strings.Repeat("  ", nestLevel))
	fmt.Fprintf(writer, "- %s\n", msg.Message)
	for _, trace := range msg.Nested {
		trace.printTrace(writer, nestLevel+1)
	}
}

type messageStatus int

const (
	accepted messageStatus = iota
	ignored
	rejected
)

func (m messageStatus) String() string {
	//exhaustive:enforce
	switch m {
	case accepted:
		return "ACCEPTED"
	case ignored:
		return "IGNORED"
	case rejected:
		return "REJECTED"
	}

	panic("unreachable")
}

type messageResult[T any] struct {
	Message T
	Status  messageStatus
	Trace   TraceMessage
}

func (result messageResult[T]) prettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "Message '%+v' - %s:\n", result.Message, result.Status)

	result.Trace.printTrace(writer, 1)
	fmt.Fprintln(writer, "")
}
