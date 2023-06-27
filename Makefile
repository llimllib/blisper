GOFILES=$(shell find . -iname '*.go')
# $(info [$(GOFILES)])

bin/blisper: $(GOFILES)
	go build -o bin/blisper .

.PHONY: watch
watch:
	modd
