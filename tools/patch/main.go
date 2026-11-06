package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/wii-tools/wadlib"
)

func main() {
	if len(os.Args) < 6 {
		fmt.Fprintf(os.Stderr, "usage: %s <input.wad> <output.wad> --old <host> --new <host> [...]\n", os.Args[0])
		os.Exit(1)
	}
	in := os.Args[1]
	out := os.Args[2]
	args := os.Args[3:]
	var olds, news []string
	for i := 0; i < len(args); i += 4 {
		if i+3 >= len(args) || args[i] != "--old" || args[i+2] != "--new" {
			fmt.Fprintln(os.Stderr, "error: expected repeatable --old <host> --new <host> pairs")
			os.Exit(1)
		}
		olds = append(olds, args[i+1])
		news = append(news, args[i+3])
	}
	for i := range olds {
		if len(olds[i]) == 0 {
			fmt.Fprintln(os.Stderr, "error: old hostname must not be empty")
			os.Exit(1)
		}
		if len(news[i]) > len(olds[i]) {
			fmt.Fprintf(os.Stderr, "error: new hostname %q longer than old %q\n", news[i], olds[i])
			os.Exit(1)
		}
	}
	w, err := wadlib.LoadWADFromFile(in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load WAD: %v\n", err)
		os.Exit(1)
	}
	total := 0
	for idx := range w.Data {
		data, err := w.GetContent(idx)
		if err != nil {
			fmt.Printf("warning: skipping content %d: cannot decrypt: %v\n", idx, err)
			continue
		}
		n := 0
		for k := range olds {
			ob := []byte(olds[k])
			nb := make([]byte, len(ob))
			copy(nb, news[k])
			c := bytes.Count(data, ob)
			if c > 0 {
				data = bytes.ReplaceAll(data, ob, nb)
				n += c
			}
		}
		if n > 0 {
			if err := w.UpdateContent(idx, data); err != nil {
				fmt.Fprintf(os.Stderr, "error: update content %d: %v\n", idx, err)
				os.Exit(1)
			}
		}
		fmt.Printf("content %d: %d replacements\n", idx, n)
		total += n
	}
	fmt.Printf("total: %d replacements\n", total)
	raw, err := w.GetWAD(w.Header.WADType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: pack WAD: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, raw, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
		os.Exit(1)
	}
}
