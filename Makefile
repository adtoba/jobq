DATABASE_URL ?= postgres://jobq:jobq@localhost:5433/jobq?sslmode=disable
export DATABASE_URL

.PHONY: db-up db-down db-reset psql vet test test-race check

db-up:      ## start Postgres and wait until healthy
	docker compose up -d --wait

db-down:    ## stop Postgres (keeps data)
	docker compose down

db-reset:   ## wipe Postgres data and start fresh
	docker compose down -v
	$(MAKE) db-up

psql:       ## open a psql shell
	docker compose exec postgres psql -U jobq jobq

vet:
	go vet ./...

test:
	go test ./...

test-race:  ## what CI runs; always use this before committing
	go test -race -count=1 ./...

check: vet test-race
