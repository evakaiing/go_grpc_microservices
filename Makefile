PROTO_PATH = api/service.proto
OUT_PATH = pkg/api

.PHONY: gen
gen:
	mkdir -p $(OUT_PATH)
	protoc --go_out=$(OUT_PATH) --go_opt=paths=source_relative \
		--go-grpc_out=$(OUT_PATH) --go-grpc_opt=paths=source_relative \
		--proto_path=api \
		service.proto

.PHONY: run
run:
	go run cmd/server/main.go

.PHONY: test
test:
	go test -v -race ./...
