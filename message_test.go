package main

import (
	"bytes"
	"testing"

	"upspin.io/config"
	"upspin.io/upspin"
)

func TestMessage(t *testing.T) {
	var user upspin.UserName = "rwcarlsen@gmail.com"
	var parent MsgName
	body := bytes.NewBufferString("hello conversing world")

	config, err := config.InitConfig(nil)
	if err != nil {
		t.Fatal(err)
	}

	m := NewMessage(user, "mytitle", parent, body)
	payload, err := m.Sign(config)
	if err != nil {
		t.Fatal(err)
	}

	// verify message integrity
	err = m.Verify(config)
	if err != nil {
		t.Fatal(err)
	}

	// reparse message from payload and verify integrity
	mparsed, err := ParseMessage(bytes.NewBufferString(payload))
	if err != nil {
		t.Fatal(err)
	}
	err = mparsed.Verify(config)
	if err != nil {
		t.Fatal(err)
	}
	payload2, err := mparsed.Payload()
	if err != nil {
		t.Fatal(err)
	}

	if payload != payload2 {
		t.Errorf("payloads not equal:\n\npayload1:\n%v\n\npayload2:\n%v\n", payload, payload2)
	} else {
		t.Logf("payload:\n%v\n", payload)
	}
}
