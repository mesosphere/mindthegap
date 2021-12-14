package main

import (
	"io"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/mesosphere/mindthegap/cmd"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	logrus.SetOutput(io.Discard)

	cmd.Execute()
}
