// Package domain is the routing model: a Route sends a Domain to one
// Container and port.
package domain

// Route is where an Application's Domain is served from.
type Route struct {
	ApplicationID uint64
	Domain        string
	Container     string
	Port          int
}
