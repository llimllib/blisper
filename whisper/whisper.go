package whisper

// #include <stdio.h>
// #include <whisper.h>
// #cgo LDFLAGS: -lwhisper -lm -lstdc++
// #cgo darwin LDFLAGS: -framework Accelerate
import "C"

import (
	"fmt"
	"time"

	"github.com/llimllib/blisper/fakestdio"
)

var SAMPLE_RATE = uint32(C.WHISPER_SAMPLE_RATE)

type Whisper struct {
	Model string
	Quiet bool

	// Stream is currently unused as streaming causes the code to cross the C
	// boundary very often and leads to serious performance issues
	Stream bool
}

type Segment struct {
	Text  string
	Start time.Duration
	End   time.Duration
}

func New(modelPath string, quiet bool) *Whisper {
	return &Whisper{
		Model: modelPath,
		Quiet: quiet,
	}
}

func (b *Whisper) Transcribe(samples []float32) ([]Segment, error) {
	var stdio *fakestdio.FakeStdOutErr
	var err error
	if b.Quiet {
		// init_from_file and whisper_full_n_segments write to stderr without
		// any config options. Soak up stderr so they don't go into our output
		stdio, err = fakestdio.New()
		if err != nil {
			return nil, err
		}
	}

	// construct a whisper context from a model file
	ctx := C.whisper_init_from_file(C.CString(b.Model))
	if ctx == nil {
		return nil, fmt.Errorf("unable to init context")
	}

	if !b.Quiet {
		fmt.Printf("%s\n", C.GoString(C.whisper_print_system_info()))
	}

	// get the default set of params
	wparams := C.whisper_full_default_params(C.WHISPER_SAMPLING_GREEDY)

	// run the model against the input
	//
	// https://github.com/ggerganov/whisper.cpp/blob/72deb41e/bindings/go/whisper.go#L328
	if res := C.whisper_full(
		(*C.struct_whisper_context)(ctx),
		(C.struct_whisper_full_params)(wparams),
		(*C.float)(&samples[0]),
		C.int(len(samples))); res != 0 {
		return nil, fmt.Errorf("Failure to convert, code %d", res)
	}

	// https://github.com/ggerganov/whisper.cpp/blob/72deb41e/bindings/go/pkg/whisper/context.go#L203
	// https://github.com/ggerganov/whisper.cpp/blob/72deb41e/examples/main/main.cpp#L309
	// https://github.com/ggerganov/whisper.cpp/blob/72deb41e/examples/main/main.cpp#L897
	n_segments := int(C.whisper_full_n_segments((*C.struct_whisper_context)(ctx)))
	segments := make([]Segment, 0, n_segments)

	if b.Quiet {
		if _, _, err = stdio.ReadAndRestore(); err != nil {
			return nil, err
		}
	}

	for i := 0; i < n_segments; i++ {
		text := C.GoString(C.whisper_full_get_segment_text((*C.struct_whisper_context)(ctx), C.int(i)))
		t0 := C.whisper_full_get_segment_t0((*C.struct_whisper_context)(ctx), C.int(i))
		t1 := C.whisper_full_get_segment_t1((*C.struct_whisper_context)(ctx), C.int(i))

		segments = append(segments, Segment{
			Text:  text,
			Start: time.Duration(t0) * time.Millisecond * 10,
			End:   time.Duration(t1) * time.Millisecond * 10,
		})
	}

	return segments, nil
}
