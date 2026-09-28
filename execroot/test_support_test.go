package execroot

import (
	"kamaji/obj"
	"kamaji/rt"
)

var testRuntime rt.Runtime

func CreateExecRootDir(selected obj.ExecTarget) error {
	return (&Preparer{Runtime: &testRuntime}).CreateExecRootDir(selected)
}
func CopyThirdPartyIntoExecRootDir() error {
	return (&Preparer{Runtime: &testRuntime}).CopyThirdPartyIntoExecRootDir()
}

func extractTarGz(src, dst string) error {
	return (&Preparer{Runtime: &testRuntime}).extractTarGz(src, dst)
}
