package helloworld

//go:generate protoc -I . -I ../../../third --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. ./helloworld.proto
