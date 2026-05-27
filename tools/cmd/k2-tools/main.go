package main

import (
	"os"
	"path/filepath"

	"github.com/wyvernzora/k2/tools/internal/toolcli"
)

func main() {
	os.Exit(toolcli.Main(filepath.Base(os.Args[0]), os.Args[1:]))
}
