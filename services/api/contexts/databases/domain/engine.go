package domain

import (
	"net/url"
	"slices"
	"strconv"
)

// Engine is what kind of Database it is.
type Engine string

const (
	PostgreSQL Engine = "postgresql"
	MySQL      Engine = "mysql"
	MariaDB    Engine = "mariadb"
	Redis      Engine = "redis"
	Valkey     Engine = "valkey"
	MongoDB    Engine = "mongodb"
)

// Engines lists every Engine, in the order the dashboard shows them.
var Engines = []Engine{PostgreSQL, MySQL, MariaDB, Redis, Valkey, MongoDB}

func (e Engine) Valid() bool { return slices.Contains(Engines, e) }

func engineNames() []string {
	out := make([]string, len(Engines))
	for i, e := range Engines {
		out[i] = string(e)
	}
	return out
}

// EngineSpec is how one Engine runs.
type EngineSpec struct {
	// Repository is the image without its tag; the Database version is
	// the tag.
	Repository     string
	DefaultVersion string
	Port           int
	// DataPath is where the volume is mounted.
	DataPath string
	// RootPassword is whether a separate root password is generated.
	RootPassword bool
	scheme       string
}

var specs = map[Engine]EngineSpec{
	PostgreSQL: {Repository: "docker.io/library/postgres", DefaultVersion: "18-alpine", Port: 5432, DataPath: "/var/lib/postgresql/data", scheme: "postgres"},
	MySQL:      {Repository: "docker.io/library/mysql", DefaultVersion: "8.4", Port: 3306, DataPath: "/var/lib/mysql", RootPassword: true, scheme: "mysql"},
	MariaDB:    {Repository: "docker.io/library/mariadb", DefaultVersion: "11", Port: 3306, DataPath: "/var/lib/mysql", RootPassword: true, scheme: "mysql"},
	Redis:      {Repository: "docker.io/library/redis", DefaultVersion: "8-alpine", Port: 6379, DataPath: "/data", scheme: "redis"},
	Valkey:     {Repository: "docker.io/valkey/valkey", DefaultVersion: "8-alpine", Port: 6379, DataPath: "/data", scheme: "redis"},
	MongoDB:    {Repository: "docker.io/library/mongo", DefaultVersion: "8", Port: 27017, DataPath: "/data/db", scheme: "mongodb"},
}

// Spec returns the Engine's spec; the zero spec for an invalid Engine.
func (e Engine) Spec() EngineSpec { return specs[e] }

// Image is the full image reference of a Database.
func (d Database) Image() string { return d.Engine.Spec().Repository + ":" + d.Version }

// Env is the Container's environment. Every secret the Container needs is
// passed here, never on its command line.
func (d Database) Env() map[string]string {
	c := d.Credentials
	switch d.Engine {
	case PostgreSQL:
		return map[string]string{
			"POSTGRES_USER": c.Username, "POSTGRES_PASSWORD": c.Password, "POSTGRES_DB": c.DatabaseName,
			// A directory below the mount point works for every version,
			// including 18, whose image moved its default data path.
			"PGDATA": d.Engine.Spec().DataPath + "/pgdata",
		}
	case MySQL:
		return map[string]string{
			"MYSQL_ROOT_PASSWORD": c.RootPassword, "MYSQL_USER": c.Username,
			"MYSQL_PASSWORD": c.Password, "MYSQL_DATABASE": c.DatabaseName,
		}
	case MariaDB:
		return map[string]string{
			"MARIADB_ROOT_PASSWORD": c.RootPassword, "MARIADB_USER": c.Username,
			"MARIADB_PASSWORD": c.Password, "MARIADB_DATABASE": c.DatabaseName,
		}
	case Redis, Valkey:
		return map[string]string{"REDIS_PASSWORD": c.Password}
	case MongoDB:
		return map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": c.Username, "MONGO_INITDB_ROOT_PASSWORD": c.Password,
			"MONGO_INITDB_DATABASE": c.DatabaseName,
		}
	}
	return nil
}

// Command overrides the image's command, or is nil to keep it. Redis and
// Valkey read their password from the environment through sh, and still go
// through the image's entrypoint so they drop root.
func (d Database) Command() []string {
	switch d.Engine {
	case Redis:
		return []string{"sh", "-c", `exec docker-entrypoint.sh redis-server --requirepass "$REDIS_PASSWORD" --appendonly yes`}
	case Valkey:
		return []string{"sh", "-c", `exec docker-entrypoint.sh valkey-server --requirepass "$REDIS_PASSWORD" --appendonly yes`}
	}
	return nil
}

// ReadinessProbe is run inside the Container; exit code 0 means it answers.
// Each connects over TCP, so the temporary server the images run while
// initialising (socket only) does not count as ready.
func (d Database) ReadinessProbe() []string {
	switch d.Engine {
	case PostgreSQL:
		return []string{"sh", "-c", `pg_isready -q -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"`}
	case MySQL:
		return []string{"sh", "-c", `mysqladmin ping --silent -h 127.0.0.1 -uroot -p"$MYSQL_ROOT_PASSWORD"`}
	case MariaDB:
		return []string{"sh", "-c", `mariadb-admin ping --silent -h 127.0.0.1 -uroot -p"$MARIADB_ROOT_PASSWORD"`}
	case Redis:
		return []string{"sh", "-c", `redis-cli -h 127.0.0.1 -a "$REDIS_PASSWORD" --no-auth-warning ping | grep -q PONG`}
	case Valkey:
		return []string{"sh", "-c", `valkey-cli -h 127.0.0.1 -a "$REDIS_PASSWORD" --no-auth-warning ping | grep -q PONG`}
	case MongoDB:
		return []string{"sh", "-c", `mongosh --quiet --host 127.0.0.1 -u "$MONGO_INITDB_ROOT_USERNAME" -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --eval 'db.runCommand({ping: 1}).ok' | grep -q 1`}
	}
	return nil
}

// URL is the connection URL of the Database on host:port: the Internal URL
// with its Container name and Engine port, the Public URL with the public
// host and Public port.
func (d Database) URL(host string, port int) string {
	c := d.Credentials
	u := url.URL{Scheme: d.Engine.Spec().scheme, Host: host + ":" + strconv.Itoa(port)}
	switch d.Engine {
	case Redis, Valkey:
		u.User = url.UserPassword("default", c.Password)
		u.Path = "/0"
	case MongoDB:
		u.User = url.UserPassword(c.Username, c.Password)
		u.Path = "/" + c.DatabaseName
		u.RawQuery = "authSource=admin"
	default:
		u.User = url.UserPassword(c.Username, c.Password)
		u.Path = "/" + c.DatabaseName
	}
	return u.String()
}

// InternalURL is the URL Applications use on the bakery network.
func (d Database) InternalURL() string {
	return d.URL(ContainerName(d.Slug), d.Engine.Spec().Port)
}

// PublicURL is the URL through the Public port on host, or "" without one.
func (d Database) PublicURL(host string) string {
	if d.PublicPort == 0 {
		return ""
	}
	return d.URL(host, d.PublicPort)
}
