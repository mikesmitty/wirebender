//go:build rp

package main

import (
	"machine"
	"machine/usb/cdc"
	"reflect"

	"bending-rodriguez/pkg/event"
	"bending-rodriguez/pkg/serial"
)

type Serials struct {
	serials map[string]*serial.Serial
}

func InitSerials(bus *event.EventBus, pub string, subs ...string) *Serials {
	ptty := serial.NewPseudoSerial()
	serialOptions := map[string]serial.Serialer{
		"serial": ptty,
	}

	if reflect.DeepEqual(machine.Serial, machine.USBCDC) {
		serialOptions["uart"] = machine.DefaultUART
	} else {
		usbcdc := machine.USBCDC
		if usbcdc == nil {
			usbcdc = cdc.New()
		}
		serialOptions["usb"] = usbcdc
	}

	const bufSize = 16
	serials := make(map[string]*serial.Serial)
	for id, s := range serialOptions {
		s.Configure(machine.UARTConfig{})
		serials[id] = serial.NewSerial(s, bus.NewEventClient(id, pub, bufSize))
		serials[id].Event.Subscribe(subs...)
	}
	return &Serials{serials: serials}
}

func (s *Serials) Update() {
	for _, s := range s.serials {
		s.Update()
	}
}
