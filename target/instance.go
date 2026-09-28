package target

import (
	"kamaji/rt"
	"net/http"
)

type Manager struct {
	Runtime *rt.Runtime
	Client  *http.Client
}
