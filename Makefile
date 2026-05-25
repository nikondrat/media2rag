.PHONY: build run test clean

build:
	go build -o media2rag ./cmd/media2rag

run: build
	./media2rag

test:
	go test ./...

clean:
	rm -f media2rag
