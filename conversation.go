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

// ConvPath builds an upspin path for the given user's directory for the named conversation with
// the passed path elements appended on.
func ConvPath(u upspin.UserName, title string, paths ...string) upspin.PathName {
	return upspin.PathName(path.Join(append([]string{string(u), ConverseDir, title}, paths...)...))
}

func AddFile(c upspin.Config, title, fname string, r io.Reader) (err error) {
	cl := client.New(c)
	f, err := cl.Create(ConvPath(c.UserName(), title, fname))
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

func ListConversations(c upspin.Config) ([]upspin.PathName, error) {
	cl := client.New(cfg)
	pth := string(ConvPath(c.UserName(), "*"))

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
	Title        string
	Participants []upspin.UserName
}

func ReadConversation(c upspin.Config, title string) (*Conversation, error) {
	cl := client.New(c)
	ents, err := cl.Glob(string(ConvPath(c.UserName(), title, msgPrefix+"*-*."+msgExtension)))
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation messages: %v", err)
	}

	conv := &Conversation{Title: title}
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

	ac, err := readAccess(c, title)
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
	if c.Title == "" {
		return errors.New("cannot add participant to conversation with no title")
	} else if c.isParticipant(u) {
		return nil
	}

	cl := client.New(cfg)
	pth := ConvPath(cfg.UserName(), c.Title, "Access")

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

func readAccess(c upspin.Config, title string) (*access.Access, error) {
	cl := client.New(c)
	pth := ConvPath(c.UserName(), title, "Access")
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

func (c *Conversation) String() string {
	var buf bytes.Buffer
	for i, msg := range c.Messages {
		fmt.Fprintf(&buf, msgSeparator, i+1)
		fmt.Fprint(&buf, msg)
	}
	return buf.String()
}

func (c *Conversation) Add(user upspin.UserName, body io.Reader) *Message {
	m := NewMessage(user, c.Title, c.nextParent(), body)
	c.Messages = append(c.Messages, m)
	return m
}

func (c *Conversation) nextParent() MsgName {
	if len(c.Messages) == 0 {
		return MsgName("")
	}
	return c.Messages[len(c.Messages)-1].Name()
}
