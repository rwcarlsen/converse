package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"strconv"
	"strings"
	"time"

	"upspin.io/bind"
	"upspin.io/factotum"
	"upspin.io/upspin"
)

const msgPrefix = "msg"
const msgSigHeader = "\n\n-------------------------- SIGNATURE ---------------------------"
const msgHeaderMarker = "-------------------------- END HEADER --------------------------\n\n"
const sigBase = 16

// Lookup returns the public key for a given upspin user using the key server
// endpoint contained in the given upspin config.
func Lookup(config upspin.Config, name upspin.UserName) (key upspin.PublicKey, err error) {
	keyserv, err := bind.KeyServer(config, config.KeyEndpoint())
	if err != nil {
		return key, err
	}
	user, err := keyserv.Lookup(name)
	if err != nil {
		return key, err
	}
	return user.PublicKey, nil
}

type MsgName string

func NewMsgName(user upspin.UserName, num int) MsgName {
	return MsgName(fmt.Sprintf("%v%v-%v.md", msgPrefix, num, user))
}

func ParseMsgName(name string) MsgName {
	mn := MsgName(name)
	mn.Number()
	return mn
}

func (n MsgName) NextName(user upspin.UserName) MsgName {
	if n == "" {
		return NewMsgName(user, 1)
	}
	return NewMsgName(user, n.Number()+1)
}

func (n MsgName) User() upspin.UserName {
	user := n[strings.Index(string(n), "-")+1 : strings.LastIndex(string(n), ".")]
	return upspin.UserName(user)
}

func (n MsgName) Number() int {
	numStr := n[len(msgPrefix):strings.Index(string(n), ".")]
	num, err := strconv.Atoi(string(numStr))
	if err != nil {
		panic("invalid message name '" + n + "'")
	}
	return num
}

type Message struct {
	Author  upspin.UserName
	Time    time.Time
	Parent  MsgName `json:"ParentMessage"`
	body    io.Reader
	content string
	sig     upspin.Signature
}

func ParseMessage(r io.Reader) (*Message, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := string(data)

	// parse/separate header, body, footer
	i := strings.Index(s, msgHeaderMarker)
	if i < 0 {
		return nil, errors.New("failed to find header while parsing message")
	}

	j := strings.LastIndex(s, msgSigHeader)
	if j < 0 {
		return nil, errors.New("failed to find signature while parsing message")
	}

	header := s[:i]
	footer := s[j+len(msgSigHeader):]
	content := s[i+len(msgHeaderMarker) : j]

	m := &Message{content: content}

	// parse header
	err = json.Unmarshal([]byte(header), &m)
	if err != nil {
		return nil, errors.New("malformed message header: " + err.Error())
	}

	// parse crypto signature
	sigText := strings.TrimSpace(footer)
	parts := strings.Split(sigText, "\n")
	if len(parts) != 2 {
		return nil, errors.New("found malformed signature while parsing message")
	}
	rs, ss := parts[0], parts[1]

	rint, sint := new(big.Int), new(big.Int)
	rint, success := rint.SetString(rs, sigBase)
	if !success {
		return nil, errors.New("invalid signature format found while parsing message")
	}
	sint, success = sint.SetString(ss, sigBase)
	if !success {
		return nil, errors.New("invalid signature format found while parsing message")
	}

	m.sig = upspin.Signature{R: rint, S: sint}
	return m, nil
}

func (m *Message) Verify(c upspin.Config) error {
	key, err := Lookup(c, m.Author)
	if err != nil {
		return fmt.Errorf("failed to discover message author's public key: %v", err)
	}
	return factotum.Verify(m.contentHash(), m.sig, key)
}

func NewMessage(author upspin.UserName, parent MsgName, body io.Reader) *Message {
	return &Message{Author: author, Parent: parent, body: body, Time: time.Now()}
}

func (m *Message) Name() MsgName { return m.Parent.NextName(m.Author) }

func (m *Message) contentHash() []byte {
	h := sha256.Sum256([]byte(m.payloadNoSig()))
	return h[:]
}

func (m *Message) payloadNoSig() string {
	var header = struct {
		Author        string
		Time          time.Time
		ParentMessage string
	}{string(m.Author), m.Time, string(m.Parent)}
	data, err := json.MarshalIndent(header, "", "    ")
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s\n%v", data, msgHeaderMarker)
	buf.WriteString(m.content)
	return buf.String()
}
func (m *Message) payloadSigOnly() string {
	return fmt.Sprintf("%v\n%x\n%x\n", msgSigHeader, m.sig.R, m.sig.S)
}

func (m *Message) Payload() (string, error) {
	if m.sig.R == nil {
		return "", errors.New("cannog provide payload of an unsigned message")
	}
	return m.payloadNoSig() + m.payloadSigOnly(), nil
}

func (m *Message) Sign(c upspin.Config) (payload string, err error) {
	if m.sig.R != nil {
		return "", errors.New("message has already been signed")
	}

	data, err := ioutil.ReadAll(m.body)
	if err != nil {
		return "", err
	}
	m.content = string(data)

	m.sig, err = c.Factotum().Sign(m.contentHash())
	if err != nil {
		return "", err
	}

	return m.payloadNoSig() + m.payloadSigOnly(), nil
}
