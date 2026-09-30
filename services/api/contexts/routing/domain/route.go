// Package domain is the routing model: a Route sends an Application's
// Domains to one Container and port.
package domain

// Route is where an Application's Domains are served from.
type Route struct {
	ApplicationID uint64
	// Domains has at least one Domain, the primary first.
	Domains   []string
	Container string
	Port      int
}

// DashboardRoute serves Bakery's own dashboard on its Domain: /api/* goes to
// the API, everything else to the dashboard. It comes from configuration and
// is never stored: it has no Application and must exist before any deploy.
type DashboardRoute struct {
	Domain string
	API    string // dial address, e.g. bakery-api:4910
	Web    string // dial address, e.g. bakery-web:80
}
