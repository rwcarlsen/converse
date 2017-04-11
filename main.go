package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "upspin.io/key/remote" // needed for KeyServer operations
)

func init() { log.SetFlags(log.Lshortfile) }

const usage = `converse [flags...] <subcmd>
`

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal(usage)
	}

	switch cmd := flag.Arg(0); cmd {
	case "list":
		list(cmd, flag.Args()[1:])
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

func printUsage(cmd, usage, msg string) {
	log.Fatalf("%v. Usage:\n    %v %v\n", msg, cmd, usage)
}

func list(cmd string, args []string) {
	const usage = `<conversation-directory>`
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	err := fs.Parse(args)
	check(err)

	if fs.NArg() != 1 {
		printUsage(cmd, usage, "Wrong number of arguments")
	}

	convName := fs.Arg(0)
	f, err := os.Open(convName)
	check(err)
	defer f.Close()

	names, err := f.Readdirnames(0)
	check(err)
	sortMsgs(names)

	for _, name := range names {
		data, err := ioutil.ReadFile(filepath.Join(convName, name))
		check(err)
		fmt.Println(string(data))
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
