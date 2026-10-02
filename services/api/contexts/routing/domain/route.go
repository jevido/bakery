// Package domain is the routing model: a Route sends an Application's
// Domains to one Container and port.
package domain

// Route is where an Application's Domains are served from.
type Route struct {
	ApplicationID uint64
	// ServerID is the Application's Target server, 0 for the Local server;
	// only that Server's Proxy serves the Route.
	ServerID uint64
	// Domains has at least one Domain, the primary first.
	Domains   []string
	Container string
	Port      int
	// Settings are the Application's Route settings, stored on their own
	// and attached for rendering.
	Settings RouteSettings
	// Derived marks Domains nobody set explicitly (a Preview domain): any
	// of them that another Route, Service route or the Dashboard Route
	// serves is left out.
	Derived bool
}

// PreviewRoute is where a Preview's Preview domain is served from. One per
// (Application, Preview number), on the Application's Server.
type PreviewRoute struct {
	ApplicationID uint64
	Preview       int
	ServerID      uint64
	Domains       []string
	Container     string
	Port          int
}

// Route is the Preview route in the shape Render takes: the Application's
// Response headers and Basic auth, never a Www redirect, and Derived.
func (r PreviewRoute) Route(settings RouteSettings) Route {
	settings.WwwRedirect = WwwOff
	return Route{
		ApplicationID: r.ApplicationID, ServerID: r.ServerID, Domains: r.Domains, Container: r.Container, Port: r.Port,
		Settings: settings, Derived: true,
	}
}

// DashboardRoute serves Bakery's own dashboard on its Domain: /api/* goes to
// the API, everything else to the dashboard. It comes from configuration and
// is never stored: it has no Application and must exist before any deploy.
type DashboardRoute struct {
	Domain string
	API    string // dial address, e.g. bakery-api:4910
	Web    string // dial address, e.g. bakery-web:80
}

// ServiceRoute is where a Public Component of a Service is served from.
// One per (Service, Component).
type ServiceRoute struct {
	ServiceID uint64
	Component string
	Domains   []string
	Container string
	Port      int
}

// Route is the Service route in the shape Render takes, with default Route
// settings (Services have none of their own yet).
func (r ServiceRoute) Route() Route {
	return Route{Domains: r.Domains, Container: r.Container, Port: r.Port, Settings: DefaultRouteSettings(0)}
}
