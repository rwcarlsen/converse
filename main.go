package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"upspin.io/client"
	"upspin.io/config"
	_ "upspin.io/key/remote" // needed for KeyServer operations
	"upspin.io/upspin"
)

func init() {
	log.SetFlags(0)
	flag.Usage = func() { fmt.Print(usage) }
}

const usage = `converse [flags...] <subcmd>
	show     print all messages in a conversation
	create   create and print signed message 
	send     send a created message
`

const defaultConfigPath = "$HOME/upspin/config"

var configPath = flag.String("config", defaultConfigPath, "upspin config file")

var cfg upspin.Config

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal(usage)
	}

	loadConfig(*configPath)

	switch cmd := flag.Arg(0); cmd {
	case "show":
		show(cmd, flag.Args()[1:])
	case "create":
		create(cmd, flag.Args()[1:])
	case "send":
		send(cmd, flag.Args()[1:])
	default:
		log.Fatalf("unrecognized subcommand '%v'", cmd)
	}
}

func send(cmd string, args []string) {
	const usage = `<user>...`
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.Usage = printUsage(fs, cmd, usage)
	err := fs.Parse(args)
	check(err)

	if fs.NArg() < 1 {
		log.Println("Need at least 1 argument.")
		fs.Usage()
	}

	m, err := ParseMessage(os.Stdin)
	check(err)

	users := map[string]struct{}{string(cfg.UserName()): struct{}{}}
	for _, user := range fs.Args() {
		users[user] = struct{}{}
	}

	cl := client.New(cfg)
	for user := range users {
		dir := path.Join(user, ConverseDir, m.Title)
		pth := path.Join(dir, string(m.Name()))
		cl.MakeDirectory(upspin.PathName(dir))

		if _, err := cl.Lookup(upspin.PathName(dir), false); err != nil {
			log.Fatalf("failed to create conversation directory %v", dir)
		}

		f, err := cl.Create(upspin.PathName(pth))
		check(err)
		payload, err := m.Payload()
		check(err)
		log.Print("sending to ", user)
		_, err = io.Copy(f, bytes.NewBufferString(payload))
		check(err)
		check(f.Close())
	}
}

func create(cmd string, args []string) {
	const usage = `<title> <message-text>...`
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.Usage = printUsage(fs, cmd, usage)
	err := fs.Parse(args)
	check(err)

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

	parent, err := nextParent(title)
	if err != nil {
		log.Print(err)
	}

	m := NewMessage(cfg.UserName(), title, parent, bytes.NewBufferString(msg))
	payload, err := m.Sign(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(payload)
}

func show(cmd string, args []string) {
	const usage = `<conversation-name>`
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.Usage = printUsage(fs, cmd, usage)
	err := fs.Parse(args)
	check(err)

	if fs.NArg() != 1 {
		log.Println("Wrong number of arguments")
		fs.Usage()
	}

	conv, err := readConversation(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(conv)
}

func loadConfig(path string) {
	var err error
	if path == defaultConfigPath {
		cfg, err = config.InitConfig(nil)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		f, err := os.Open(path)
		if err != nil {
			log.Fatalf("config file not found: %v", err)
		}
		defer f.Close()
		cfg, err = config.InitConfig(f)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func nextParent(conversationName string) (MsgName, error) {
	if conversationName == "" {
		return "", nil
	}

	conv, err := readConversation(conversationName)
	if err != nil {
		return "", fmt.Errorf("no existing conversation named '%v' found", conversationName)
	} else if len(conv.Messages) == 0 {
		return "", nil
	}
	return conv.Messages[len(conv.Messages)-1].Name(), nil
}

func readConversation(convName string) (*Conversation, error) {
	name := path.Join(string(cfg.UserName()), ConverseDir, convName)
	return ReadConversation(cfg, upspin.PathName(name))
}

func printUsage(fs *flag.FlagSet, cmd, usage string) func() {
	return func() {
		log.Printf("Usage:\n    %v %v\nOptions:\n", cmd, usage)
		fs.PrintDefaults()
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
