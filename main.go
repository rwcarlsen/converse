package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"upspin.io/config"
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

	switch cmd := flag.Arg(0); cmd {
	case "show":
		show(cmd, flag.Args()[1:])
	case "verify":
		verify(cmd, flag.Args()[1:])
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

	for user := range users {
		log.Print("sending to ", user)
		if err := m.Send(cfg, upspin.UserName(user)); err != nil {
			log.Printf("send to %v failed", user)
		}
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

	conv, err := ReadConversation(cfg, ConvPath(user, title))
	if err != nil {
		log.Printf("no existing conversation named '%v' found", title)
	}

	m := conv.Add(user, bytes.NewBufferString(msg))
	m.Title = title
	payload, err := m.Sign(cfg)
	check(err)
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

	conv, err := ReadConversation(cfg, ConvPath(user, fs.Arg(0)))
	check(err)
	fmt.Print(conv)
}

func verify(cmd string, args []string) {
	const usage = `<conversation-name>`
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.Usage = printUsage(fs, cmd, usage)
	err := fs.Parse(args)
	check(err)

	if fs.NArg() != 1 {
		log.Println("Wrong number of arguments")
		fs.Usage()
	}

	conv, err := ReadConversation(cfg, ConvPath(user, fs.Arg(0)))
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
