// Command fakesign-channel fakesigns the ticket and TMDs of an already imported channel package.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: fakesign-channel <channels-root> <title-id>")
		os.Exit(2)
	}
	m, err := channelpkg.Load(os.Args[1], os.Args[2])
	if err != nil {
		fail(err)
	}
	dir := filepath.Join(os.Args[1], m.TitleID)
	for _, name := range []string{"tmd", fmt.Sprintf("tmd.%d", m.Version), "ticket.bin"} {
		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			fail(err)
		}
		if name == "ticket.bin" {
			err = channelpkg.FakesignTicket(b)
		} else {
			err = channelpkg.FakesignTMD(b)
		}
		if err != nil {
			fail(fmt.Errorf("%s: %w", name, err))
		}
		st, _ := os.Stat(path)
		tmp := path + ".fakesign"
		if err = os.WriteFile(tmp, b, st.Mode().Perm()); err != nil {
			fail(err)
		}
		if err = os.Rename(tmp, path); err != nil {
			fail(err)
		}
		fmt.Println("fakesigned", path)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
