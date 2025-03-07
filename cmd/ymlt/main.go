package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/olastor/ymlt"
	"gopkg.in/yaml.v3"
)

var USAGE = `Usage:
  -d, --defaults string  Set default values
  -v                     Display version
  -h, --help             Display help`

func isInputFromPipe() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	return fileInfo.Mode()&os.ModeCharDevice == 0
}

func main() {
	var (
		defaultsFlag string
		helpFlag     bool
		versionFlag  bool
	)

	flag.Usage = func() { fmt.Fprintf(os.Stderr, "%s\n", USAGE) }
	flag.StringVar(&defaultsFlag, "defaults", "", "")
	flag.StringVar(&defaultsFlag, "d", "", "")
	flag.BoolVar(&versionFlag, "v", false, "")
	flag.BoolVar(&helpFlag, "h", false, "")
	flag.BoolVar(&helpFlag, "help", false, "")

	flag.Parse()

	var decoder *yaml.Decoder

	if isInputFromPipe() {
		decoder = yaml.NewDecoder(bufio.NewReader(os.Stdin))
	} else {
		yamlFile, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening file: %v\n", err)
			os.Exit(1)
		}

		decoder = yaml.NewDecoder(yamlFile)
	}

	var defaults []byte
	if defaultsFlag != "" {
		var err error
		defaults, err = os.ReadFile(defaultsFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read defaults file: %v", err)
			os.Exit(1)
		}
	}

	i := 0
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}

		if i > 0 {
			os.Stdout.Write([]byte("---\n"))
		}

		err := ymlt.Apply(&doc, &ymlt.Config{Defaults: defaults})
		if err != nil {
			panic(err)
		}

		buf := bytes.Buffer{}
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		err = enc.Encode(&doc)
		if err != nil {
			panic(err)
		}
		os.Stdout.Write([]byte(fmt.Sprintf("%s", buf.String())))
		i++
	}
}
