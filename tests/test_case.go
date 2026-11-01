package tests

import (
	"github.com/goravel/framework/testing"

	"neoskin/bootstrap"
)

func init() {
	bootstrap.Boot()
}

type TestCase struct {
	testing.TestCase
}
