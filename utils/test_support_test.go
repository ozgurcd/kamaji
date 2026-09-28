package utils

var globalShareDir = "/usr/local/share/kamaji"

func CopyRulesToGlobalDir() error { return (&Installer{Directory: globalShareDir}).Install() }
