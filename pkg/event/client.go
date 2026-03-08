package event

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/mikesmitty/wirebender/pkg/topic"
)

const (
	defaultBufferSize = 3
)

type EventClient struct {
	ClientID   string
	Bus        Bus
	Receive    chan Event
	DefaultPub string
	Topics     []string
}

func (eb *EventBus) NewEventClient(clientID string, defaultPub string, bufSize ...int) *EventClient {
	if len(bufSize) == 0 {
		bufSize = append(bufSize, defaultBufferSize)
	}
	return &EventClient{
		ClientID:   clientID,
		Bus:        eb,
		Receive:    make(chan Event, bufSize[0]),
		DefaultPub: defaultPub,
	}
}

func (cl *EventClient) Subscribe(topics ...string) {
	for _, topic := range topics {
		cl.Topics = append(cl.Topics, topic)
		cl.Bus.Subscribe(topic, cl.ClientID, cl.Receive)
	}
}

func (cl *EventClient) Unsubscribe(leaveTopics ...string) {
	j := 0
	for _, topic := range cl.Topics {
		if slices.Contains(leaveTopics, topic) {
			cl.Bus.Unsubscribe(topic, cl.ClientID)
		} else {
			cl.Topics[j] = topic
			j++
		}
	}
	cl.Topics = cl.Topics[:j]
}

func (cl *EventClient) UnsubscribeFromAll() {
	for _, topic := range cl.Topics {
		cl.Bus.Unsubscribe(topic, cl.ClientID)
	}
	cl.Topics = []string{}
}

func (cl *EventClient) Publish(data any) {
	cl.Bus.Publish(cl.DefaultPub, cl.ClientID, data)
}

func (cl *EventClient) PublishTo(topic string, data any) {
	cl.Bus.Publish(topic, cl.ClientID, data)
}

func (cl *EventClient) diag(topic, msg string, args ...any) {
	buf := new(bytes.Buffer)
	buf.Write([]byte("<* "))
	buf.Write(fmt.Appendf(nil, msg, args...))
	buf.Write([]byte(" *>"))
	cl.Bus.Publish(topic, cl.ClientID, buf)
}

func (cl *EventClient) Diag(msg string, args ...any) {
	cl.diag(topic.BroadcastDiag, msg, args...)
}

func (cl *EventClient) Debug(msg string, args ...any) {
	cl.diag(topic.BroadcastDebug, msg, args...)
}

type Bus interface {
	Subscribe(topic, clientID string, ch chan Event)
	Unsubscribe(topic, clientID string)
	Publish(topic, clientID string, data any)
}
