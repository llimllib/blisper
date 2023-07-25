GOFILES=$(shell find . -iname '*.go')
# $(info [$(GOFILES)])

LIBWHISPER=$(shell brew --prefix libwhisper)

# export tells make to pass the variables to all subshells
# https://www.gnu.org/software/make/manual/html_node/Variables_002fRecursion.html
export C_INCLUDE_PATH=$(LIBWHISPER)/include
export LIBRARY_PATH=$(LIBWHISPER)/lib

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
	# install statticcheck if not installed
	if ! command -v staticcheck &>/dev/null; then go install honnef.co/go/tools/cmd/staticcheck@latest; fi
	staticcheck ./...
	go vet ./...
