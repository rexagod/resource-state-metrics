package resourcestatemetrics_test

import (
	"os"
	"testing"
)

const (
	MainPort = "RSM_MAIN_PORT"
	SelfPort = "RSM_SELF_PORT"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
