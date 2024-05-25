**gochat**

A simple chat server and client bundle for your local home network.

- TUI with colors
  - color coded chat messages
  - user list with user names and ids
- Login screen
  - username selection
  - host selection
    - automatically scans for hosts on your local network to select from - no memorizing or typing IP addresses!
    - only works for Class A `10.X.X.X` private network addressing (`192.168.X.X` not supported)
- Press ESC to disconnect

![gochat](https://github.com/kvitebjorn/gochat/assets/7317556/60e5c467-f00f-4b71-a50c-e7b3764fea2b)

**Build commands**

- **_Native Linux_**

```
go build -o ./bin/server/gochat-server ./cmd/server && go build -o ./bin/client/gochat-client ./cmd/client
```

- **_Native Windows_**

```
go build -o ./bin/server/gochat-server.exe ./cmd/server && go build -o ./bin/client/gochat-client.exe ./cmd/client
```

- **_Cross-platform (Windows example)_**

```
env GOOS=windows GOARCH=amd64 go build -o ./bin/server/gochat-server-win.exe ./cmd/server && env GOOS=windows GOARCH=amd64 go build -o ./bin/client/gochat-client-win.exe ./cmd/client
```

- **_Cross-platform (Raspberry Pi 5 example)_**

```
env GOOS=linux GOARCH=arm GOARM=5 go build -o ./bin/server/gochat-server-arm ./cmd/server && env GOOS=linux GOARCH=arm GOARM=5 go build -o ./bin/client/gochat-client-arm ./cmd/client
```
