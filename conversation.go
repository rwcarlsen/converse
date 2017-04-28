package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/russross/blackfriday"

	"upspin.io/access"
	"upspin.io/client"
	"upspin.io/upspin"
)

const msgSeparator = "---------------------------- msg %v -------------------------------\n"
const DefaultConverseDir = "conversations"

func ConvPath(u upspin.UserName, title string) upspin.PathName {
	return Join(DefaultRoot(u), title)
}

//
func DefaultRoot(u upspin.UserName) upspin.PathName {
	return Join(upspin.PathName(u), DefaultConverseDir)
}

func ListConversations(cl upspin.Client, root upspin.PathName) ([]upspin.PathName, error) {
	pth := string(Join(root, "*"))

	ents, err := cl.Glob(pth)
	if err != nil {
		return nil, err
	}

	var convs []upspin.PathName

	for _, ent := range ents {
		convs = append(convs, ent.SignedName)
	}
	return convs, nil
}

type Conversation struct {
	Messages     []*Message
	Participants []upspin.UserName
	Location     upspin.PathName
	title        string
}

func NewConversation(root upspin.PathName, title string) *Conversation {
	return &Conversation{title: title, Location: Join(root, title)}
}

func ReadConversation(cl upspin.Client, dir upspin.PathName) (*Conversation, error) {
	conv := &Conversation{Location: dir}
	if err := conv.Init(cl); err != nil {
		return nil, err
	}

	ents, err := cl.Glob(string(Join(dir, msgPrefix+"*-*."+msgExtension)))
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation messages: %v", err)
	}

	for _, ent := range ents {
		m, err := ReadMessage(cl, ent.SignedName)
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

	ac, err := readAccess(cl, dir)
	if err != nil {
		// read through messages to discover participants
		for _, m := range conv.Messages {
			conv.Participants = append(conv.Participants, m.Author)
		}
	} else {
		// load participants from access file
		conv.Participants, err = ac.Users(access.Read, func(p upspin.PathName) ([]byte, error) { return cl.Get(p) })
		if err != nil {
			return nil, err
		}
	}

	return conv, nil
}

func (c *Conversation) SetTitle(title string) error {
	if c.Title() != "" {
		return errors.New("cannot set title on a titled conversation")
	}
	c.title = title
	return nil
}

func (c *Conversation) Init(cl upspin.Client) error {
	return MakeDirs(cl, c.Location)
}

func (c *Conversation) isParticipant(u upspin.UserName) bool {
	for _, user := range c.Participants {
		if user == u {
			return true
		}
	}
	return false
}

func (c *Conversation) RenderHtml() []byte {
	var buf bytes.Buffer
	for i, m := range c.Messages {
		fmt.Fprintf(&buf, "\n------------ *%v on %v (msg %v)* ------------\n\n%v\n",
			m.Author, m.Time.Format(time.UnixDate), i+1, m.Content())
	}
	return blackfriday.MarkdownCommon(buf.Bytes())
}

func (c *Conversation) Title() string {
	if len(c.Messages) == 0 {
		return c.title
	}
	return c.Messages[0].Title
}

func (c *Conversation) String() string {
	var buf bytes.Buffer
	for i, msg := range c.Messages {
		fmt.Fprintf(&buf, msgSeparator, i+1)
		fmt.Fprint(&buf, msg)
	}
	return buf.String()
}

func (c *Conversation) Add(user upspin.UserName, body io.Reader) *Message {
	if c.Title() == "" {
		panic("cannot add messages to untitled conversation")
	}
	m := NewMessage(user, c.Title(), c.nextParent(), body)
	c.Messages = append(c.Messages, m)
	return m
}

func (c *Conversation) nextParent() MsgName {
	if len(c.Messages) == 0 {
		return MsgName("")
	}
	return c.Messages[len(c.Messages)-1].Name()
}

func (c *Conversation) hasAccess(cl upspin.Client, u upspin.UserName) bool {
	ac, err := readAccess(cl, c.Location)
	if err != nil {
		return false
	}

	users, err := ac.Users(access.Read, func(p upspin.PathName) ([]byte, error) { return cl.Get(p) })
	if err != nil {
		return false
	}
	for _, hasAccess := range users {
		if u == hasAccess {
			return true
		}
	}
	return false
}

func (c *Conversation) AddParticipant(cfg upspin.Config, u upspin.UserName) error {
	if !c.isParticipant(u) {
		c.Participants = append(c.Participants, u)
	}

	cl := client.New(cfg)
	if c.hasAccess(cl, u) {
		return nil
	}

	pth := Join(c.Location, "Access")

	var data []byte

	_, err := cl.Lookup(pth, false)
	if err != nil {
		// create access file
		data = []byte(fmt.Sprintf("*: %v", cfg.UserName()))
	} else {
		data, err = cl.Get(pth)
		if err != nil {
			return err
		}
	}

	data = append(data, []byte(fmt.Sprintf("\nread,create,list: %v", u))...)

	_, err = cl.Put(pth, data)
	if err != nil {
		return err
	}

	return nil
}

func (c *Conversation) Publish(cl upspin.Client) error {
	if len(c.Messages) == 0 {
		return errors.New("cannot publish a conversation without no messages")
	}

	pth := Join(c.Location, "index.html")
	_, err := cl.Put(pth, c.RenderHtml())
	if err != nil {
		return fmt.Errorf("failed to create published 'index.html' file: %v", err)
	}
	return nil
}
