package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
	"os"
)

func main() {
	slug := flag.String("sd-app", "", "app slug compiled into this reviewed SD installer channel")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: import-channel [-sd-app slug] homebrew-channel.wad output-store")
		os.Exit(2)
	}
	var m channelpkg.Manifest
	var e error
	if *slug == "" {
		m, e = channelpkg.Import(flag.Arg(0), flag.Arg(1))
	} else {
		m, e = channelpkg.ImportSDInstaller(flag.Arg(0), flag.Arg(1), *slug)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(m)
}
