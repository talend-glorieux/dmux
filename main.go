package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	tag := flag.String("t", "", "Tags to apply to the produced Docker image.")
	branch := flag.String("branch", "", "Override git branches defined on Dockerfile")
	flag.Parse()
	builder, err := NewBuilder(flag.Arg(0))
	checkError(err)
	err = builder.Build(*tag, *branch)
	checkError(err)
}

func checkError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
