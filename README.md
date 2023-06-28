# Blisper

A [whisper.cpp](https://github.com/ggerganov/whisper.cpp/tree/master) CLI wrapper in go.

## building

To build

- clone [whisper.cpp](https://github.com/ggerganov/whisper.cpp/tree/master)
- from the `bindings/go` directory of that repo, run `make whisper`
  - This will build `libwhisper.a` into the root directory of that repository
- set the `C_INCLUDE_PATH` and `LIBRARY_PATH` environment variables to the directory where you cloned whisper.cpp
- run `make`

## usage

```
	fmt.Println(`Usage: blisper [OPTIONS] <input-audio> <output-transcript>

Use whisper.cpp to transcribe the <input-audio> file into <output-transcript>

OPTIONS

  -config:       print the config for this app
  -format <fmt>: the output format to use. Defaults to "srt"
  -help, -h:     print this help
  -model:        the size of the whisper model to use. Defaults to "small"
  -stream:       if passed, stream output to stdout
  -verbose, -v:  print verbose output

MODELS

  Valid models are: tiny.en, tiny, base.en, base, small.en, small, medium.en, medium, large-v1, large

  Blisper will automatically download a model if you do not already have it on your system

FORMATS

  Valid subtitle formats are srt, ssa, stl, ttml, and vtt. The default format is srt
```

## status

It's currently functional, but the golang API for whisper.cpp is incomplete and slow; it won't do parallel processing and it lacks advanced functionality not exposed in the API.

There are general discussions about the API in [this thread](https://github.com/ggerganov/whisper.cpp/discussions/312)

The main reason I want my own CLI for whsiper is that the binary built by `make` in the whisper.cpp repository expects you to manage your own models; I think that's cumbersome and user-unfriendly. Ideally I'd like to have a binary that can be `brew install`ed, and this repository is a step towards it.

However, until the go binary can do parallel processing and access more of the functionality in whisper.cpp, I don't think this will reach a high enough level of quality to make it workable.

## TODO

- figure out how to build whisper.cpp as a `.so` shared library file that can be distributed along with the repository, so users don't have to clone it and build it themselves
  - some discussion [here](https://github.com/ggerganov/whisper.cpp/discussions/312#discussioncomment-6271170) and [here](https://github.com/ggerganov/whisper.cpp/discussions/312#discussioncomment-6271439)
  - there is a `libwhsiper.so` target in the Makefile [here](https://github.com/ggerganov/whisper.cpp/blob/master/Makefile#L255)
- see how far I could get by using the "low-level" go binding rather than the high-level one
  - can parallel processing be succesfully enabled? It's currently [disabled here](https://github.com/ggerganov/whisper.cpp/blob/72deb41eb26300f71c50febe29db8ffcce09256c/bindings/go/pkg/whisper/context.go#L169)
- stream data from the WAV into the processing function, rather than doing it all in batch
  - would save memory and increase speed
- more configuration options
  - smaller chunks
  - [here are the options](https://github.com/ggerganov/whisper.cpp/blob/72deb41eb26300f71c50febe29db8ffcce09256c/examples/main/main.cpp#L118) the default binary supports
