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
	"path"
	"strconv"
	"strings"
	"time"

	"upspin.io/bind"
	"upspin.io/client"
	"upspin.io/factotum"
	"upspin.io/upspin"
)

const (
	msgPrefix    = "msg"
	msgExtension = "txt"
)

const (
	msgSigHeader    = "\n\n-------------------------- SIGNATURE ---------------------------"
	msgHeaderMarker = "-------------------------- END HEADER --------------------------\n\n"
	sigBase         = 16
)

// lookup returns the public key for a given upspin user using the key server
// endpoint contained in the given upspin config.
func lookup(config upspin.Config, name upspin.UserName) (key upspin.PublicKey, err error) {
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
	return MsgName(fmt.Sprintf("%v%v-%v.%v", msgPrefix, num, user, msgExtension))
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
	i := strings.Index(string(n), ".")
	i2 := strings.Index(string(n), "-")
	if i2 < i {
		i = i2
	}
	if i < 0 {
		panic("invalid message name '" + n + "'")
	}
	numStr := n[len(msgPrefix):i]
	num, err := strconv.Atoi(string(numStr))
	if err != nil {
		panic("invalid message name '" + n + "'")
	}
	return num
}

type Message struct {
	Author upspin.UserName
	// Title represents the name of this message's conversation
	Title   string
	Time    time.Time
	Parent  MsgName `json:"ParentMessage"`
	Body    io.Reader
	content string
	sig     upspin.Signature
}

func ReadMessage(cl upspin.Client, path upspin.PathName) (*Message, error) {
	f, err := cl.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseMessage(f)
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

func (m *Message) Send(c upspin.Config, recipient upspin.UserName) (err error) {
	dir := ConvPath(recipient, m.Title)
	pth := path.Join(string(dir), string(m.Name()))

	cl := client.New(c)

	cl.MakeDirectory(upspin.PathName(dir))
	if _, err = cl.Lookup(upspin.PathName(dir), false); err != nil {
		return fmt.Errorf("failed to create conversation directory %v", dir)
	}

	f, err := cl.Create(upspin.PathName(pth))
	if err != nil {
		return err
	}
	defer func() { err = f.Close() }()

	payload, err := m.Payload()
	if err != nil {
		return err
	}

	_, err = io.Copy(f, bytes.NewBufferString(payload))
	return err
}

func (m *Message) Verify(c upspin.Config) error {
	key, err := lookup(c, m.Author)
	if err != nil {
		return fmt.Errorf("failed to discover message author's public key: %v", err)
	}
	return factotum.Verify(m.contentHash(), m.sig, key)
}

func NewMessage(author upspin.UserName, title string, parent MsgName, body io.Reader) *Message {
	return &Message{Author: author, Title: title, Parent: parent, Body: body, Time: time.Now()}
}

func (m *Message) Name() MsgName { return m.Parent.NextName(m.Author) }

func (m *Message) String() string {
	content := strings.Replace(m.content, "\n", "\n    ", -1)
	return fmt.Sprintf("%v on %v\n    %v\n",
		m.Author, m.Time.Format(time.UnixDate), content)
}

func (m *Message) contentHash() []byte {
	h := sha256.Sum256([]byte(m.payloadNoSig()))
	return h[:]
}

func (m *Message) payloadNoSig() string {
	var header = struct {
		Author        string
		Time          time.Time
		ParentMessage string
		Title         string
	}{string(m.Author), m.Time, string(m.Parent), m.Title}
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

	data, err := ioutil.ReadAll(m.Body)
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
