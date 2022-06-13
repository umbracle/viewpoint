
sszgen:
	sszgen --path internal/server/proto/structs.go
	sszgen --path internal/genesis.structs.go
	
protoc:
	protoc --go_out=. --go-grpc_out=. ./internal/server/proto/*.proto
