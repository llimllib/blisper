package main

// #include <stdio.h>
// #include <whisper/whisper.h>
// #cgo LDFLAGS: -L./whisper -l whisper
// #cgo LDFLAGS: -Wl,-rpath,@executable_path/whisper/
import "C"
import "fmt"

func main() {
	// The function call we're going to try and reproduce is:
	// struct whisper_context * ctx = whisper_init_from_file(params.model.c_str());

	path := C.CString("/Users/llimllib/.local/share/blisper/ggml-small.bin")
	C.whisper_init_from_file(path)

	fmt.Printf("hi\n")
}
