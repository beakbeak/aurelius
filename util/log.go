package util

import (
	"io/ioutil"
	"log"
	"os"
)

var (
	DebugEnabled = true
	Debug        *log.Logger
	Noise        *log.Logger
)

func init() {
	Debug = log.New(os.Stdout, "DEBUG: ", log.Ltime|log.Lmicroseconds|log.Ldate|log.Lshortfile)
	Noise = log.New(os.Stdout, "NOISE: ", log.Ltime|log.Lmicroseconds|log.Ldate|log.Lshortfile)
}

func SetLogLevel(level int) {
	if level < 2 {
		DebugEnabled = false
		Debug.SetOutput(ioutil.Discard)
		Debug.SetFlags(0)
	}
	if level < 3 {
		Noise.SetOutput(ioutil.Discard)
		Noise.SetFlags(0)
	}
}
