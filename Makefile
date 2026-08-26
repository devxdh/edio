.PHONY: build test install clean dev demo demo-tui

build:
	go build -o bin/edio ./cmd/edio

test:
	go test -v ./...

install:
	go install ./cmd/edio

clean:
	rm -rf bin/ .git/edio /tmp/edio-demo-session

dev:
	go run ./cmd/edio/

demo: build
	@echo "--- Running Edio Utilitarian Engine Demo ---"
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

demo-tui:
	@./scripts/run_demo.sh
