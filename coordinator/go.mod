module github.com/two-phase-commit-demo/coordinator

go 1.23.2

require (
	github.com/Alvintan0712/two-phase-commit-demo/shared v0.1.0
	google.golang.org/grpc v1.69.2
)

require (
	github.com/go-zookeeper/zk v1.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241015192408-796eee8c2d53 // indirect
	google.golang.org/protobuf v1.36.1 // indirect
)

replace github.com/Alvintan0712/two-phase-commit-demo/shared => ../shared
