package chanassert

import (
	"fmt"
	"io"
	"strings"
)

type messageMode int

const (
	info messageMode = iota
	debug
)

type TraceMessage struct {
	Message string
	Nested  []TraceMessage
	Mode    messageMode
}

func newInfoTrace(message string, nested ...TraceMessage) TraceMessage {
	return TraceMessage{Message: message, Nested: nested, Mode: info}
}

func newDebugTrace(message string, nested ...TraceMessage) TraceMessage {
	return TraceMessage{Message: message, Nested: nested, Mode: debug}
}

var levelPrefixes = []rune{'-', '*', '+', '>'}

func (msg TraceMessage) PrintTrace(writer io.Writer, nestLevel int) {
	fmt.Fprint(writer, strings.Repeat("  ", nestLevel+1))

	prefix := levelPrefixes[nestLevel%len(levelPrefixes)]

	//exhaustive:enforce
	switch msg.Mode {
	case info:
		fmt.Fprintf(writer, "%c %s\n", prefix, msg.Message)
	case debug:
		fmt.Fprintf(writer, "%c [DEBUG] %s\n", prefix, msg.Message)
	}

	for _, trace := range msg.Nested {
		trace.PrintTrace(writer, nestLevel+1)
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

func (result messageResult[T]) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "Message '%+v' - %s:\n", result.Message, result.Status)

	result.Trace.PrintTrace(writer, 0)
	fmt.Fprintln(writer, "")
}
