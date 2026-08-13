// Package main 是 scu 命令行工具的入口。
package main

import (
	"fmt"
	"os"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
