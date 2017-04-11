package main

import (
	"bytes"
	"fmt"
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

	m := NewMessage(user, parent, body)
	payload, err := m.Sign(config)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%s", payload)

	pubkey, err := Lookup(config, user)
	if err != nil {
		t.Fatal(err)
	}

	// verify message integrity
	err = m.Verify(pubkey)
	if err != nil {
		t.Fatal(err)
	}

	// reparse message from payload and verify integrity
	mparsed, err := ParseMessage(payload)
	if err != nil {
		t.Fatal(err)
	}
	err = mparsed.Verify(pubkey)
	if err != nil {
		t.Fatal(err)
	}
}
