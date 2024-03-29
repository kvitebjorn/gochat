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

- ***Native***
```
go build -o ./bin/server/gochat-server ./cmd/server && go build -o ./bin/client/gochat-client ./cmd/client
```

- ***Cross-platform (Windows example)***
```
env GOOS=windows GOARCH=amd64 go build -o ./bin/server/gochat-server-win.exe ./cmd/server && env GOOS=windows GOARCH=amd64 go build -o ./bin/client/gochat-client-win.exe ./cmd/client
```

- ***Cross-platform (Raspberry Pi 5 example)***
```
env GOOS=linux GOARCH=arm GOARM=5 go build -o ./bin/server/gochat-server-arm ./cmd/server && env GOOS=linux GOARCH=arm GOARM=5 go build -o ./bin/client/gochat-client-arm ./cmd/client
```
