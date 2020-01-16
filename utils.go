package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func prompt(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(question, " [Y/n] ")
	answer, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "yes" || answer == "y" {
		return true
	}
	return false
}
