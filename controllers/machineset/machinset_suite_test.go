package machineset_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMachinset(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Machinset Suite")
}
