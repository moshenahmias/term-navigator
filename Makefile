.PHONY: build clean test

build:
	go build -o bin/tn ./cmd/tn

clean:
	rm -rf bin/

test:
	go test ./cmd/tn

run:
	./bin/tn