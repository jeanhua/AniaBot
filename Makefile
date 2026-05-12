ifeq ($(OS),Windows_NT)
linux:
	set GOOS=linux&& set GOARCH=amd64&& go build -ldflags="-s -w" -o ./build/AniaBot ./cmd/

windows:
	set GOOS=windows&& set GOARCH=amd64&& go build -ldflags="-s -w" -o ./build/AniaBot.exe ./cmd/
else
linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./build/AniaBot ./cmd/

windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./build/AniaBot.exe ./cmd/
endif

clean:
	rm -rf ./build/*
