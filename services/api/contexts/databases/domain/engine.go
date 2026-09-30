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
	// Backups is whether the Engine is backed up.
	Backups bool
	scheme  string
}

var specs = map[Engine]EngineSpec{
	PostgreSQL: {Repository: "docker.io/library/postgres", DefaultVersion: "18-alpine", Port: 5432, DataPath: "/var/lib/postgresql/data", Backups: true, scheme: "postgres"},
	MySQL:      {Repository: "docker.io/library/mysql", DefaultVersion: "8.4", Port: 3306, DataPath: "/var/lib/mysql", RootPassword: true, Backups: true, scheme: "mysql"},
	MariaDB:    {Repository: "docker.io/library/mariadb", DefaultVersion: "11", Port: 3306, DataPath: "/var/lib/mysql", RootPassword: true, Backups: true, scheme: "mysql"},
	Redis:      {Repository: "docker.io/library/redis", DefaultVersion: "8-alpine", Port: 6379, DataPath: "/data", scheme: "redis"},
	Valkey:     {Repository: "docker.io/valkey/valkey", DefaultVersion: "8-alpine", Port: 6379, DataPath: "/data", scheme: "redis"},
	MongoDB:    {Repository: "docker.io/library/mongo", DefaultVersion: "8", Port: 27017, DataPath: "/data/db", Backups: true, scheme: "mongodb"},
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

// restoreDir is where a Backup file is copied into the Container to be
// restored; RestoreCommand removes it afterwards.
const restoreDir = "/tmp"

// BackupExt is the file extension of the Engine's Backups.
func (d Database) BackupExt() string {
	switch d.Engine {
	case PostgreSQL:
		return ".dump"
	case MySQL, MariaDB:
		return ".sql.gz"
	case MongoDB:
		return ".archive.gz"
	}
	return ""
}

// RestoreFile is the name the Backup file gets inside the Container, in
// RestoreDir.
func (d Database) RestoreFile() string { return "bakery-restore" + d.BackupExt() }

// RestoreDir is the directory in the Container the Backup file is copied to.
func (d Database) RestoreDir() string { return restoreDir }

// DumpGzip is whether the dump's output is gzipped by Bakery (the Engine's
// tool does not compress).
func (d Database) DumpGzip() bool { return d.Engine == MySQL || d.Engine == MariaDB }

// DumpCommand writes a logical dump of the Database to stdout. Credentials
// come from the Container's environment, never from the command line.
func (d Database) DumpCommand() []string {
	switch d.Engine {
	case PostgreSQL:
		return []string{"sh", "-c", `PGPASSWORD="$POSTGRES_PASSWORD" exec pg_dump -Fc -h 127.0.0.1 -U "$POSTGRES_USER" "$POSTGRES_DB"`}
	case MySQL:
		return []string{"sh", "-c", `MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump --single-transaction --routines --triggers --events -h 127.0.0.1 -uroot --databases "$MYSQL_DATABASE"`}
	case MariaDB:
		return []string{"sh", "-c", `MYSQL_PWD="$MARIADB_ROOT_PASSWORD" exec mariadb-dump --single-transaction --routines --triggers --events -h 127.0.0.1 -uroot --databases "$MARIADB_DATABASE"`}
	case MongoDB:
		return []string{"sh", "-c", `exec mongodump --quiet --archive --gzip --host 127.0.0.1 -u "$MONGO_INITDB_ROOT_USERNAME" -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --db "$MONGO_INITDB_DATABASE"`}
	}
	return nil
}

// RestoreCommand replaces the Database's data with the Backup file at
// RestoreDir/RestoreFile, then removes the file whatever the outcome.
func (d Database) RestoreCommand() []string {
	f := "'" + restoreDir + "/" + d.RestoreFile() + "'"
	var restore string
	switch d.Engine {
	case PostgreSQL:
		restore = `PGPASSWORD="$POSTGRES_PASSWORD" pg_restore --clean --if-exists --no-owner --exit-on-error -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" ` + f
	case MySQL:
		restore = `gunzip -t ` + f + ` && gunzip -c ` + f + ` | MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h 127.0.0.1 -uroot`
	case MariaDB:
		restore = `gunzip -t ` + f + ` && gunzip -c ` + f + ` | MYSQL_PWD="$MARIADB_ROOT_PASSWORD" mariadb -h 127.0.0.1 -uroot`
	case MongoDB:
		restore = `mongorestore --quiet --archive=` + f + ` --gzip --drop --host 127.0.0.1 -u "$MONGO_INITDB_ROOT_USERNAME" -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin`
	default:
		return nil
	}
	// A pipeline's status is its last command's, and not every image's sh
	// has pipefail: gunzip -t checks the whole file before mysql reads it.
	return []string{"sh", "-c", restore + "; code=$?; rm -f " + f + "; exit $code"}
}
