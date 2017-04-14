package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/toqueteos/webbrowser"

	"upspin.io/client"
	"upspin.io/cmd/cacheserver/cacheutil"
	"upspin.io/config"
	"upspin.io/transports"
	"upspin.io/upspin"
)

func init() {
	log.SetFlags(0)
	flag.Usage = func() { fmt.Print(usage) }
}

const usage = `converse [flags...] <subcmd>
	list     list all existing conversations
	show     print all messages in a conversation
	publish  render+save a conversation as html in its dir
	download download an entire conversation
	create   create and print signed message 
	send     send a created message
	addfile  add a file to a conversation
	verify   verify integrity of all messages in a conversation
`

const defaultConfigPath = "$HOME/upspin/config"

var configPath = flag.String("config", defaultConfigPath, "upspin config file")

var cfg upspin.Config
var user upspin.UserName

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal(usage)
	}

	loadConfig(*configPath)
	cmd := flag.Arg(0)
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)

	switch cmd {
	case "download":
		download(fs, cmd, flag.Args()[1:])
	case "publish":
		publish(fs, cmd, flag.Args()[1:])
	case "addfile":
		addfile(fs, cmd, flag.Args()[1:])
	case "show":
		show(fs, cmd, flag.Args()[1:])
	case "verify":
		verify(fs, cmd, flag.Args()[1:])
	case "create":
		create(fs, cmd, flag.Args()[1:])
	case "send":
		send(fs, cmd, flag.Args()[1:])
	case "list":
		list(fs, cmd, flag.Args()[1:])
	default:
		log.Fatalf("unrecognized subcommand '%v'", cmd)
	}
}

func send(fs *flag.FlagSet, cmd string, args []string) {
	const usage = `[<title> <message>...]`
	var users = fs.String("to", "", "comma-separated recipient(s) of the message")
	fs.Usage = mkUsage(fs, cmd, usage)
	fs.Parse(args)

	if 0 < fs.NArg() && fs.NArg() < 2 {
		log.Println("Need zero or 2+ arguments")
		fs.Usage()
	}

	var err error
	var m *Message
	var conv *Conversation
	if fs.NArg() == 0 {
		m, err = ParseMessage(os.Stdin)
		check(err)
		conv, err = ReadConversation(cfg, m.Title)
		check(err)
	} else {
		title := fs.Arg(0)
		conv, err = ReadConversation(cfg, title)
		check(err)
		m = conv.Add(user, bytes.NewBufferString(strings.Join(fs.Args()[1:], " ")))
	}

	for _, user := range strings.Split(*users, ",") {
		if user != "" {
			if err := conv.AddParticipant(cfg, upspin.UserName(user)); err != nil {
				log.Printf("failed to add %v to conversation: %v", user, err)
			}
		}
	}

	for _, user := range conv.Participants {
		log.Print("sending to ", user)
		if err := m.Send(cfg, upspin.UserName(user)); err != nil {
			log.Printf("send to %v failed", user)
		}
	}

	check(conv.Publish(cfg))
}

func publish(fs *flag.FlagSet, cmd string, args []string) {
	const usage = `<title>`
	fs.Usage = mkUsage(fs, cmd, usage)
	fs.Parse(args)

	if fs.NArg() != 1 {
		log.Println("Need exactly 1 argument")
		fs.Usage()
	}
	title := fs.Arg(0)

	conv, err := ReadConversation(cfg, title)
	check(err)
	check(conv.Publish(cfg))
}

func list(fs *flag.FlagSet, cmd string, args []string) {
	const usage = ``
	fs.Usage = mkUsage(fs, cmd, usage)
	fs.Parse(args)

	if fs.NArg() != 0 {
		log.Println("Takes no arguments")
		fs.Usage()
	}

	convs, err := ListConversations(cfg)
	check(err)

	for _, conv := range convs {
		fmt.Println(conv)
	}
}

func create(fs *flag.FlagSet, cmd string, args []string) {
	const usage = `<title> <message-text>...`
	fs.Usage = mkUsage(fs, cmd, usage)
	fs.Parse(args)

	var msg string
	if fs.NArg() < 1 {
		log.Println("Need at least 1 argument.")
		fs.Usage()
	}

	title := fs.Arg(0)
	if fs.NArg() == 1 {
		data, err := ioutil.ReadAll(os.Stdin)
		check(err)
		msg = string(data)
	} else {
		msg = strings.Join(fs.Args()[1:], " ")
	}

	conv, err := ReadConversation(cfg, title)
	if err != nil {
		log.Printf("no existing conversation named '%v' found", title)
	}

	m := conv.Add(user, bytes.NewBufferString(msg))
	m.Title = title
	payload, err := m.Sign(cfg)
	check(err)
	fmt.Println(payload)
}

func show(fs *flag.FlagSet, cmd string, args []string) {
	const usage = `<conversation-name>`
	fs.Usage = mkUsage(fs, cmd, usage)
	var dohtml = fs.Bool("html", false, "render conversation messages as html")
	fs.Parse(args)

	if fs.NArg() != 1 {
		log.Println("Wrong number of arguments")
		fs.Usage()
	}

	conv, err := ReadConversation(cfg, fs.Arg(0))
	check(err)

	if *dohtml {
		fmt.Printf("%s", conv.RenderHtml())
	} else {
		fmt.Print(conv)
	}
}

func download(fs *flag.FlagSet, cmd string, args []string) {
	const usage = `<user> <conversation-name>`
	var open = fs.Bool("open", false, "open the rendered conversation in a web broser")
	fs.Usage = mkUsage(fs, cmd, usage)
	fs.Parse(args)

	if fs.NArg() != 2 {
		log.Println("Wrong number of arguments")
		fs.Usage()
	}

	user, title := upspin.UserName(fs.Arg(0)), fs.Arg(1)

	cl := client.New(cfg)
	ents, err := cl.Glob(string(ConvPath(user, title, "*")))
	check(err)

	err = os.MkdirAll(title, 0755)
	check(err)

	for _, ent := range ents {
		fpath := ent.SignedName
		fname := path.Base(string(fpath))
		if fname == "Access" {
			continue
		} else if strings.HasPrefix(fname, ".") {
			continue
		}

		data, err := cl.Get(fpath)
		if err != nil {
			log.Printf("failed to download file %v: ", fname, err)
		}
		err = ioutil.WriteFile(filepath.Join(title, fname), data, 0644)
		if err != nil {
			log.Printf("failed to write file '%v' locally: %v", fname, err)
		}
	}

	conv, err := ReadConversation(cfg, title)
	check(err)

	html := filepath.Join(title, "index.html")
	err = ioutil.WriteFile(html, conv.RenderHtml(), 0644)
	check(err)

	if *open {
		abs, err := filepath.Abs(html)
		check(err)
		fmt.Println("file://" + abs)
		err = webbrowser.Open("file://" + abs)
		check(err)
	}
}

func addfile(fs *flag.FlagSet, cmd string, args []string) {
	const usage = `<conversation-name> <file>...`
	fs.Usage = mkUsage(fs, cmd, usage)
	fs.Parse(args)

	if fs.NArg() < 2 {
		log.Println("Wrong number of arguments")
		fs.Usage()
	}

	title := fs.Arg(0)
	for _, fname := range fs.Args()[1:] {
		func() {
			f, err := os.Open(fname)
			check(err)
			defer f.Close()
			err = AddFile(cfg, title, filepath.Base(fname), f)
			check(err)
		}()
	}
}

func verify(fs *flag.FlagSet, cmd string, args []string) {
	const usage = `<conversation-name>`
	fs.Usage = mkUsage(fs, cmd, usage)
	err := fs.Parse(args)

	if fs.NArg() != 1 {
		log.Println("Wrong number of arguments")
		fs.Usage()
	}

	conv, err := ReadConversation(cfg, fs.Arg(0))
	check(err)

	for _, msg := range conv.Messages {
		err := msg.Verify(cfg)
		if err != nil {
			log.Printf("'%v' FAILED verification", msg.Name())
		} else {
			fmt.Printf("'%v' verified\n", msg.Name())
		}
	}
}

func loadConfig(path string) {
	var err error
	if path == defaultConfigPath {
		cfg, err = config.InitConfig(nil)
		check(err)
	} else {
		f, err := os.Open(path)
		if err != nil {
			log.Fatalf("config file not found: %v", err)
		}
		defer f.Close()
		cfg, err = config.InitConfig(f)
		check(err)
	}
	user = cfg.UserName()

	transports.Init(cfg)
	cacheutil.Start(cfg)
}

func mkUsage(fs *flag.FlagSet, cmd, usage string) func() {
	return func() {
		log.Printf("Usage:\n   converse %v %v\nOptions:\n", cmd, usage)
		fs.PrintDefaults()
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
