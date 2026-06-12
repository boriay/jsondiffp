package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	jsonOutput := flags.Bool("j", false, "print diff as json")
	precision := flags.Int("p", -1, "number precision: digits after decimal point")
	directRound := flags.Bool("r", false, "use direct round for number precision; default is accumulated rounding")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "usage: %s [-j] [-r] [-p digits] <left.json> <right.json>\n", os.Args[0])
		flags.PrintDefaults()
	}

	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	precisionSet := false
	flags.Visit(func(flag *flag.Flag) {
		if flag.Name == "p" {
			precisionSet = true
		}
	})
	if precisionSet && *precision < 0 {
		fmt.Fprintln(os.Stderr, "precision must be non-negative")
		os.Exit(2)
	}
	if flags.NArg() != 2 {
		flags.Usage()
		os.Exit(2)
	}

	left, err := readJSONFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read left json: %v\n", err)
		os.Exit(1)
	}

	right, err := readJSONFile(flags.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read right json: %v\n", err)
		os.Exit(1)
	}

	roundMode := RoundAccumulated
	if *directRound {
		roundMode = RoundDirect
	}
	diffs := DiffJSONWithOptions(left, right, DiffOptions{Precision: *precision, RoundMode: roundMode})
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(diffs); err != nil {
			fmt.Fprintf(os.Stderr, "encode diff: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := WriteTextDiff(os.Stdout, diffs); err != nil {
		fmt.Fprintf(os.Stderr, "write diff: %v\n", err)
		os.Exit(1)
	}
}

func readJSONFile(path string) (any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unexpected extra json value")
	}

	return value, nil
}
