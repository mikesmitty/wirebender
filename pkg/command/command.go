package command

import (
	"bytes"
	"fmt"

	"bending-rodriguez/pkg/event"
)

type HandlerFunc func(ch *CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error

type CommandHandler struct {
	handlers map[string]HandlerFunc
	Event    *event.EventClient
}

func NewCommandHandler(cl *event.EventClient) *CommandHandler {
	ch := &CommandHandler{
		Event:    cl,
		handlers: make(map[string]HandlerFunc),
	}
	return ch
}

func (ch *CommandHandler) RegisterCommandHandler(handler HandlerFunc, cmds ...string) {
	for _, cmd := range cmds {
		ch.handlers[cmd] = handler
	}
}

func (ch *CommandHandler) Update() {
	select {
	case evt := <-ch.Event.Receive:
		switch msg := evt.Data.(type) {
		case *bytes.Buffer:
			ch.handleCmdBuffer(evt, msg)
		}
	default:
	}
}

func (ch *CommandHandler) handleCmdBuffer(evt event.Event, buf *bytes.Buffer) {
	input := buf.Bytes()
	if len(input) == 0 {
		return
	}

	response, err := ch.handleCommand(evt.ClientID, input)
	if response.Len() == 0 && err == nil {
		return
	}
	if err != nil {
		response.WriteString(fmt.Sprintf("<ERROR: %v>", err))
	}
	ch.Event.Publish(response)
}

func (ch *CommandHandler) handleCommand(clientId string, input []byte) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	words := bytes.Fields(input)
	if len(words) == 0 {
		return buf, fmt.Errorf("empty command")
	}

	cmd := string(words[0])
	params := words[1:]

	if handler, ok := ch.handlers[cmd]; ok {
		err := handler(ch, buf, cmd, params)
		return buf, err
	}

	return buf, fmt.Errorf("unknown command: %s", cmd)
}
