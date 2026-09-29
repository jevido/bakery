package tests

import (
	"github.com/goravel/framework/testing"

	"github.com/jevido/bakery/services/api/bootstrap"
)

func init() {
	bootstrap.Boot()
}

type TestCase struct {
	testing.TestCase
}
