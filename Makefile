web:
	cd web && pnpm install && pnpm run build

linux:
	cd web && pnpm install && pnpm run build && \
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./build/AniaBot ./cmd/

windows:
	cd web && pnpm install && pnpm run build \
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./build/AniaBot.exe ./cmd/

clean:
	rm -rf ./build/*
