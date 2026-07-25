.PHONY: web linux windows clean

ifeq ($(OS),Windows_NT)
# ---------- Windows (PowerShell) ----------
SHELL := powershell.exe
.SHELLFLAGS := -NoProfile -ExecutionPolicy Bypass -Command

web:
	cd web; npm ci; npm run build

linux:
	cd web; npm ci; npm run build; cd ..; $$env:GOOS="linux"; $$env:GOARCH="amd64"; go build -ldflags="-s -w" -o ./build/AniaBot ./cmd/

windows:
	cd web; npm ci; npm run build; cd ..; $$env:GOOS="windows"; $$env:GOARCH="amd64"; go build -ldflags="-s -w" -o ./build/AniaBot.exe ./cmd/

clean:
	if (Test-Path ./build) { Remove-Item -Recurse -Force ./build/* }

else
# ---------- Linux / macOS ----------
web:
	cd web && npm ci && npm run build

linux:
	cd web && npm ci && npm run build && cd .. && \
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./build/AniaBot ./cmd/

windows:
	cd web && npm ci && npm run build && cd .. && \
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./build/AniaBot.exe ./cmd/

clean:
	rm -rf ./build/*

endif
