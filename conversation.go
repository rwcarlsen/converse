package main

import (
	"bytes"
	"fmt"
	"sort"

	"upspin.io/client"
	_ "upspin.io/transports"
	"upspin.io/upspin"
)

const msgSeparator = "---------------------------- msg %v -------------------------------\n"
const ConverseDir = "conversations"

type Conversation struct {
	Messages []*Message
}

func (c *Conversation) String() string {
	var buf bytes.Buffer
	for i, msg := range c.Messages {
		fmt.Fprintf(&buf, msgSeparator, i+1)
		fmt.Fprint(&buf, msg)
	}
	return buf.String()
}

func ReadConversation(c upspin.Config, name upspin.PathName) (*Conversation, error) {
	cl := client.New(c)
	ents, err := cl.Glob(string(name) + "/*")
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation messages: %v", err)
	}

	conv := &Conversation{}
	for _, ent := range ents {
		m, err := readMsg(cl, ent.SignedName)
		if err != nil {
			return nil, fmt.Errorf("failed to open message '%v': %v", ent.SignedName, err)
		}
		conv.Messages = append(conv.Messages, m)
	}

	sort.Slice(conv.Messages, func(i, j int) bool {
		mi, mj := conv.Messages[i], conv.Messages[j]
		ni, nj := mi.Name().Number(), mj.Name().Number()
		return (ni != nj && ni < nj) || (mi.Time.Unix() < mj.Time.Unix())
	})
	return conv, nil
}

func readMsg(cl upspin.Client, path upspin.PathName) (*Message, error) {
	f, err := cl.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseMessage(f)
}
