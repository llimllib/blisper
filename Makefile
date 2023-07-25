GOFILES = $(shell find . -iname '*.go')
# $(info [$(GOFILES)])

LIBWHISPER ?= $(shell brew --prefix libwhisper)
C_INCLUDE_PATH ?= $(LIBWHISPER)/include
LIBRARY_PATH ?= $(LIBWHISPER)/lib

$(info LIBWHISPER:     $(LIBWHISPER))
$(info C_INCLUDE_PATH: $(C_INCLUDE_PATH))
$(info LIBRARY_PATH:   $(LIBRARY_PATH))
$(info  )

bin/blisper: $(GOFILES)
	C_INCLUDE_PATH=${C_INCLUDE_PATH} LIBRARY_PATH=${LIBRARY_PATH} go build -o bin/blisper .

.PHONY: install
install:
	C_INCLUDE_PATH=${C_INCLUDE_PATH} LIBRARY_PATH=${LIBRARY_PATH} go install

.PHONY: watch
watch:
	modd

.PHONY: lint
lint:
	# install statticcheck if not installed
	if ! command -v staticcheck &>/dev/null; then go install honnef.co/go/tools/cmd/staticcheck@latest; fi
	C_INCLUDE_PATH=${C_INCLUDE_PATH} LIBRARY_PATH=${LIBRARY_PATH} \
		staticcheck ./... && \
		go vet ./...
