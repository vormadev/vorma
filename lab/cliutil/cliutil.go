package cliutil

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/term"
)

const (
	ColorRed   = "\033[0;31m"
	ColorGreen = "\033[0;32m"
	ColorBlue  = "\033[0;34m"
	ColorPlain = "\033[0m"
)

func NewReader() *bufio.Reader {
	return bufio.NewReader(os.Stdin)
}

func RequireYes(failMsg string) {
	Plain("(y/n) ")

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		Exit("failed to set terminal raw mode", err)
	}

	buf := make([]byte, 1)
	_, err = os.Stdin.Read(buf)

	term.Restore(fd, oldState)

	if err != nil {
		Exit("failed to read input", err)
	}

	fmt.Printf("%c", buf[0])
	NewLine()

	if buf[0] != 'y' && buf[0] != 'Y' {
		Exit(failMsg, err)
	}
}

func MustRun(cmd *exec.Cmd, failMsg string) {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		Exit(failMsg, err)
	}
}

func Cmd(cmd string, args ...string) *exec.Cmd {
	return exec.Command(cmd, args...)
}

func Exit(msg string, err error) {
	if err != nil {
		Plain("ERROR: " + msg + ": ")
		Red(fmt.Sprintf("%v", err))
	} else {
		Plain(msg)
	}
	NewLine()

	code := 0
	if err != nil {
		code = 1
	}

	os.Exit(code)
}

func Red(s string) {
	Wrap(s, startRed)
}

func Green(s string) {
	Wrap(s, startGreen)
}

func Blue(s string) {
	Wrap(s, startBlue)
}

func Plain(s string) {
	Wrap(s, startPlain)
}

func Wrap(s string, f func()) {
	f()
	fmt.Print(s)
	startPlain()
}

func NewLine() {
	fmt.Println()
}

func startRed() {
	fmt.Print(ColorRed)
}

func startGreen() {
	fmt.Print(ColorGreen)
}

func startBlue() {
	fmt.Print(ColorBlue)
}

func startPlain() {
	fmt.Print(ColorPlain)
}
