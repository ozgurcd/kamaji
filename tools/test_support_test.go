package tools

import (
	"kamaji/obj"
	"kamaji/rt"
)

var testRuntime rt.Runtime

func GetRule(selected obj.ExecTarget) (string, error) {
	return (&Context{Runtime: &testRuntime}).GetRule(selected)
}
func Unzip(src, dst string) error { return (&Context{Runtime: &testRuntime}).Unzip(src, dst) }
