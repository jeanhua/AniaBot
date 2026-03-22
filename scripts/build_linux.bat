@echo off
setlocal
set GOOS=linux
set GOARCH=amd64
cd cmd/
go build -ldflags="-s -w" -o ../build/AniaBot
endlocal