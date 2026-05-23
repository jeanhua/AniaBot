linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./build/AniaBot ./cmd/

windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./build/AniaBot.exe ./cmd/

clean:
	rm -rf ./build/*
