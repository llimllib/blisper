GOFILES=$(shell find . -iname '*.go')
# $(info [$(GOFILES)])

bin/blisper: $(GOFILES)
	go build -o bin/blisper .

.PHONY: install
install:
	go install

.PHONY: watch
watch:
	modd

.PHONY: lint
lint:
	staticcheck ./...
	go vet ./...
