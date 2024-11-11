package main

import (
	"os"
	"runtime"

	"github.com/Velocidex/fileb0x/runner"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	arg := os.Args[len(os.Args)-1:][0]

	err := runner.Process(arg)
	if err != nil {
		panic(err)
	}
}
