.PHONY: web linux windows clean

web:
	cd web && npm ci && npm run build

linux:
	cd web && npm ci && npm run build && \
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./build/AniaBot ./cmd/

windows:
	cd web && npm ci && npm run build \
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./build/AniaBot.exe ./cmd/

clean:
	rm -rf ./build/*
