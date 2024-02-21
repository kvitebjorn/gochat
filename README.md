to build the binaries, run


λ go build -o ./bin/server/gochat-server ./cmd/server

λ go build -o ./bin/client/gochat-client ./cmd/client

λ env GOOS=windows GOARCH=amd64 go build -o ./bin/client/gochat-client-win.exe ./cmd/client