.PHONY: proto clean modules

# Commands to compile Protocol Buffers for Go
proto:
	protoc --proto_path=api/proto \
		--go_out=pb --go_opt=paths=source_relative \
		--go-grpc_out=pb --go-grpc_opt=paths=source_relative \
		auth/auth.proto track/track.proto

# Clean up generated files if needed
clean:
	rm -rf pb/auth/* pb/track/*

# Download and tidy all Go modules
modules:
	go mod tidy