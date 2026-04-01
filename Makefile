.PHONY: build clean test

build:
	go build -o bin/termnav ./cmd/termnav

clean:
	rm -rf bin/

test:
	go test ./cmd/termnav

run:
	./bin/termnav