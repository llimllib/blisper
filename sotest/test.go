package main

// #include <stdio.h>
// #include <whisper/whisper.h>
// #cgo CFLAGS: -I ${SRCDIR}/whisper
// #cgo LDFLAGS: -L ${SRCDIR}/whisper -Wl,-rpath ${SRCDIR}/whisper -lwhisper
import "C"
import "fmt"

// TODO: for some reason, I absolutely cannot get the runtime to search the
// whisper directory using the `rpath` argument. I've tried many little options
// and I'm certain it's not a big change but it's killing me. I'm giving up for
// now and sticking `libwhisper.so` in the same dir as the binary

func main() {
	// The function call we're going to try and reproduce is:
	// struct whisper_context * ctx = whisper_init_from_file(params.model.c_str());

	path := C.CString("/Users/llimllib/.local/share/blisper/ggml-small.bin")
	C.whisper_init_from_file(path)

	fmt.Printf("hi\n")
}
