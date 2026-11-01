package main

import (
	"neoskin/bootstrap"
)

func main() {
	app := bootstrap.Boot()

	app.Start()
}
