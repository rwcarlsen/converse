package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

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
`

const defaultConfigPath = "$HOME/upspin/config"

var configPath = flag.String("config", defaultConfigPath, "upspin config file")

var cfg upspin.Config

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
	default:
		log.Fatalf("unrecognized command '%v'", cmd)
	}
}

func parseMsgNum(fname string) (int, error) {
	i := strings.Index(fname, ".")
	s := fname[3:i]
	i, err := strconv.Atoi(s)
	if err != nil {
		return -1, err
	}
	return i, nil
}

func sortMsgs(names []string) {
	sort.Slice(names, func(i, j int) bool {
		ni, err := parseMsgNum(names[i])
		check(err)
		nj, err := parseMsgNum(names[j])
		check(err)
		return ni < nj
	})
}

func printUsage(fs *flag.FlagSet, cmd, usage string) func() {
	return func() {
		log.Printf("Usage:\n    %v %v\nOptions:\n", cmd, usage)
		fs.PrintDefaults()
	}
}

func create(cmd string, args []string) {
	const usage = `<message-text>...`
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	var convName = fs.String("title", "", "conversation name/title for this message")
	fs.Usage = printUsage(fs, cmd, usage)
	err := fs.Parse(args)
	check(err)

	if fs.NArg() < 1 {
		log.Println("Wrong number of arguments")
		fs.Usage()
	}

	msg := strings.Join(fs.Args(), " ")

	// TODO: set parent based on any pre-existing messages in the conversation (if any)
	var parent MsgName
	if *convName != "" {
		conv, err := readConversation(*convName)
		if err != nil || len(conv.Messages) == 0 {
			log.Printf("no existing conversation named '%v' found", *convName)
		} else {
			parent = conv.Messages[len(conv.Messages)-1].Name()
		}
	}

	m := NewMessage(cfg.UserName(), parent, bytes.NewBufferString(msg))
	payload, err := m.Sign(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(payload)
}

func readConversation(convName string) (*Conversation, error) {
	name := path.Join(string(cfg.UserName()), ConverseDir, convName)
	return ReadConversation(cfg, upspin.PathName(name))
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

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
