build:
	@go build -o bin/pcshops.exe

run: build
	@bin/pcshops.exe

test:
	@go test -v ./...