all: proto rest grpc
rest:
	go build -ldflags '-w -s -extldflags "-static"' -o user-rest ./cmd/rest
grpc:
	go build -ldflags '-w -s -extldflags "-static"' -o user-rpc ./cmd/rpc
proto:
	protoc --go_out=. --go-grpc_out=. ./proto/*.proto
