# Blisper

Transcribe audio files to text very rapidly

## building

Blisper currently only supports building on systems with homebrew.

To build, run `brew install llimllib/whisper/libwhisper && make`

## usage

```
Usage: blisper [OPTIONS] <input-audio> <output-transcript>

Use whisper.cpp to transcribe the <input-audio> file into <output-transcript>

OPTIONS

  -config:       print the config for this app
  -format <fmt>: the output format to use. Defaults to "txt"
  -help, -h:     print this help
  -model, -m:    the name of the whisper model to use. Defaults to "small"
  -stream:       if passed, stream output to stdout
  -verbose, -v:  print verbose output

MODELS

  Valid models are: tiny.en, tiny, base.en, base, small.en, small, medium.en, medium, large-v1, large

  Blisper will automatically download a model if you do not already have it on your system

FORMATS

  Valid subtitle formats are srt, ssa, stl, ttml, txt, and vtt. The default format is txt
```

## status

alpha. Basically functional but not yet easy to download and build

There are general discussions about the API in [this thread](https://github.com/ggerganov/whisper.cpp/discussions/312)

The main reason I want my own CLI for whsiper is that the binary built by `make` in the whisper.cpp repository expects you to manage your own models; I think that's cumbersome and user-unfriendly. Ideally I'd like to have a binary that can be `brew install`ed, and this repository is a step towards it.

However, until the go binary can do parallel processing and access more of the functionality in whisper.cpp, I don't think this will reach a high enough level of quality to make it workable.

## thanks

many thanks to @ggerganov for [whisper.cpp](https://github.com/ggerganov/whisper.cpp/tree/master)

## TODO

- stream data from the WAV into the processing function, rather than doing it all in batch
  - would save memory and increase speed
    - unless crossing the C boundary would be too costly
- more configuration options
  - smaller chunks
  - [here are the options](https://github.com/ggerganov/whisper.cpp/blob/72deb41eb26300f71c50febe29db8ffcce09256c/examples/main/main.cpp#L118) the default binary supports
