@echo off
setlocal
set GOOS=linux
set GOARCH=amd64
cd cmd/
go build -o ../build/AniaBot
endlocal