run: build
	@./bin/auth

install:
	go get ./...
	go mod vendor
	go mod tidy
	go mod download

build: 
	go build -o bin/auth main.go

dev: 
	go run github.com/air-verse/air@v1.52.3 \
	--build.cmd "go build -o tmp/bin/main" \
	--build.bin "tmp/bin/main" \
	--build.delay "100" \
	--build.include_ext "go" \
	--build.stop_on_error "false" \
	--misc.clean_on_exit true
