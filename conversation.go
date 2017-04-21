package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"time"

	"github.com/russross/blackfriday"

	"upspin.io/access"
	"upspin.io/client"
	"upspin.io/upspin"
)

const msgSeparator = "---------------------------- msg %v -------------------------------\n"
const ConverseDir = "conversations"

func ConvPath(u upspin.UserName, title string) upspin.PathName {
	return join(RootPath(u), title)
}

func RootPath(u upspin.UserName) upspin.PathName {
	return join(upspin.PathName(u), ConverseDir)
}

// join builds an upspin path for the given upspin path and the passed path elements joined
// together.
func join(u upspin.PathName, paths ...string) upspin.PathName {
	return upspin.PathName(path.Join(append([]string{string(u)}, paths...)...))
}

func AddFile(c upspin.Config, fpath upspin.PathName, r io.Reader) (err error) {
	cl := client.New(c)
	f, err := cl.Create(fpath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := f.Close(); err == nil {
			err = err2
		}
	}()

	_, err = io.Copy(f, r)
	return err
}

func ListConversations(c upspin.Config, root upspin.PathName) ([]upspin.PathName, error) {
	cl := client.New(cfg)
	pth := string(join(root, "*"))

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
	return &Conversation{title: title, Location: join(root, title)}
}

func ReadConversation(c upspin.Config, dir upspin.PathName) (*Conversation, error) {
	cl := client.New(c)
	ents, err := cl.Glob(string(join(dir, msgPrefix+"*-*."+msgExtension)))
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation messages: %v", err)
	}

	conv := &Conversation{Location: dir}
	for _, ent := range ents {
		m, err := ReadMessage(cl, ent.SignedName)
		if err != nil {
			return nil, fmt.Errorf("failed to open message '%v': %v", ent.SignedName, err)
		}
		m.conv = conv
		conv.Messages = append(conv.Messages, m)
	}

	if len(conv.Messages) == 0 {
		return nil, fmt.Errorf("conversation '%v' has no messages", dir)
	}

	sort.Slice(conv.Messages, func(i, j int) bool {
		mi, mj := conv.Messages[i], conv.Messages[j]
		ni, nj := mi.Name().Number(), mj.Name().Number()
		return (ni != nj && ni < nj) || (mi.Time.Unix() < mj.Time.Unix())
	})

	ac, err := readAccess(c, dir)
	if err != nil {
		// read through messages to discover participants
		for _, m := range conv.Messages {
			err = conv.AddParticipant(c, m.Author)
			if err != nil {
				return nil, err
			}
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

func (c *Conversation) isParticipant(u upspin.UserName) bool {
	for _, user := range c.Participants {
		if user == u {
			return true
		}
	}
	return false
}

func (c *Conversation) AddParticipant(cfg upspin.Config, u upspin.UserName) error {
	if c.Title() == "" {
		return errors.New("cannot add participant to conversation with no title")
	} else if c.isParticipant(u) {
		return nil
	}

	cl := client.New(cfg)
	pth := join(c.Location, "Access")

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

	c.Participants = append(c.Participants, u)
	return nil
}

func readAccess(c upspin.Config, dir upspin.PathName) (*access.Access, error) {
	cl := client.New(c)
	pth := join(dir, "Access")
	data, err := cl.Get(pth)
	if err != nil {
		return nil, err
	}
	return access.Parse(pth, data)
}

// TODO: figure out how to handle relative paths to images.  The images need
// to either be in the same (relative) directory to the html as they are to
// the conversation dir message files.  Or we need to figure out some way to
// rewrite the paths in the rendered html.  Hmmm....
func (c *Conversation) RenderHtml() []byte {
	var buf bytes.Buffer
	for i, m := range c.Messages {
		fmt.Fprintf(&buf, "\n------------- *%v on %v (msg %v)* -------------\n\n%v\n",
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

func (c *Conversation) Publish(cfg upspin.Config, root upspin.PathName) error {
	if len(c.Messages) == 0 {
		return errors.New("cannot publish a conversation without no messages")
	}

	cl := client.New(cfg)
	pth := join(root, c.Title(), "index.html")
	_, err := cl.Put(pth, c.RenderHtml())
	if err != nil {
		return fmt.Errorf("failed to create published 'index.html' file: %v", err)
	}
	return nil
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
	m := NewMessage(user, c.Title(), c.nextParent(), body)
	m.conv = c
	c.Messages = append(c.Messages, m)
	return m
}

func (c *Conversation) nextParent() MsgName {
	if len(c.Messages) == 0 {
		return MsgName("")
	}
	return c.Messages[len(c.Messages)-1].Name()
}
