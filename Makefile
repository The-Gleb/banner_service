postgres:
	docker run --name banner_db -e POSTGRES_USER=banner_db -e POSTGRES_PASSWORD=banner_db -p 5434:5432 -d postgres:alpine

postgresrm:
	docker stop banner_db
	docker rm banner_db

migrateup:
	migrate -path internal/adapter/db/postgres/migration -database "postgres://banner_db:banner_db@localhost:5434/banner_db?sslmode=disable" -verbose up

migratedown:
	migrate -path internal/adapter/db/postgres/migration -database "postgres://banner_db:banner_db@localhost:5434/banner_db?sslmode=disable" -verbose down

.PHONY: postgres createdb dropdb migrateup migratedown