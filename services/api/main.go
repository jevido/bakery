package main

import (
	"github.com/jevido/bakery/services/api/bootstrap"
)

func main() {
	app := bootstrap.Boot()

	app.Start()
}
