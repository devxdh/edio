.PHONY: build test install clean dev demo cross-compile

build:
	go build -o bin/edio ./cmd/edio

test:
	go test -v ./...

install:
	go install ./cmd/edio

clean:
	rm -rf bin/ dist/ .git/edio /tmp/edio-demo-session

dev:
	go run ./cmd/edio/

cross-compile:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/edio-linux-amd64 ./cmd/edio
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/edio-linux-arm64 ./cmd/edio
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/edio-darwin-amd64 ./cmd/edio
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/edio-darwin-arm64 ./cmd/edio
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/edio-windows-amd64.exe ./cmd/edio
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o dist/edio-windows-arm64.exe ./cmd/edio
	@echo "Cross-compilation complete in dist/"

demo: build
	@echo "--- Running Edio Engine Demo ---"
	@TEST_DIR=$$(mktemp -d); \
	cd $$TEST_DIR && \
	git init -q && \
	git config user.name "Demo User" && \
	git config user.email "demo@example.com" && \
	echo "initial content" > file.txt && \
	git add file.txt && \
	git commit -m "initial commit" -q && \
	$(CURDIR)/bin/edio init && \
	echo "turn 1 changes" >> file.txt && \
	$(CURDIR)/bin/edio snapshot -m "added line in turn 1" && \
	echo "turn 2 changes" >> file.txt && \
	$(CURDIR)/bin/edio snapshot -m "added line in turn 2" && \
	$(CURDIR)/bin/edio log && \
	$(CURDIR)/bin/edio diff 2 && \
	$(CURDIR)/bin/edio restore 1 && \
	$(CURDIR)/bin/edio accept "feat: add multi-turn feature" && \
	rm -rf $$TEST_DIR
