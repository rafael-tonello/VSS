package apis

import "rtonello/vss/sources/misc"

// IApi is a minimal interface used by ControllerClientHelper. Real APIs can implement this.
type IApi interface {
	GetAPIID() string
	CheckAlive(clientID string) bool
	NotifyClient(clientID string, varsAndValues []misc.Tuple[string]) bool
}
