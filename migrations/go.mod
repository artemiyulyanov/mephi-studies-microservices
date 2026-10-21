module migrations

go 1.26.3

require (
	common v0.0.0
	github.com/gorilla/mux v1.8.1
	github.com/jackc/pgx/v5 v5.11.0
	golang.org/x/sync v0.24.0
)

require (
	github.com/golang-migrate/migrate/v4 v4.20.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/lib/pq v1.12.3 // indirect
	golang.org/x/text v0.43.0 // indirect
)

replace common => ../common
