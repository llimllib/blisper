GOFILES=$(shell find . -iname '*.go')
LIBWHISPER=$(shell brew --prefix libwhisper)
# $(info [$(GOFILES)])

bin/blisper: $(GOFILES)
	C_INCLUDE_PATH=$(LIBWHISPER)/include \
	LIBRARY_PATH=$(LIBWHISPER)/lib \
		go build -o bin/blisper .

.PHONY: install
install:
	C_INCLUDE_PATH=$(LIBWHISPER)/include \
	LIBRARY_PATH=$(LIBWHISPER)/lib \
		go install

.PHONY: watch
watch:
	modd

.PHONY: lint
lint:
	C_INCLUDE_PATH=$(LIBWHISPER)/include \
		staticcheck ./...
	go vet ./...
