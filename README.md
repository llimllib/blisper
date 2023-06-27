To build

- clone [whisper.cpp](https://github.com/ggerganov/whisper.cpp/tree/master)
- from the `bindings/go` directory of that repo, run `make whisper`
  - This will build `libwhisper.a` into the root directory of that repository
- set the `C_INCLUDE_PATH` and `LIBRARY_PATH` environment variables to the directory where you cloned whisper.cpp
- run `make`
