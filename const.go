package sqlm

const (
	MYSQL  = "mysql"
	SQLITE = "sqlite"
	// POSTGRES is the canonical protocol name, PG the accepted shorthand. The
	// value doubles as the database/sql driver name in store.NewDriver, so the
	// caller must register a driver under it (see the postgres example).
	POSTGRES = "postgres"
	PG       = "pg"
)
