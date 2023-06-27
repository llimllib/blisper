package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/exp/slog"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/go-audio/wav"
)

// get the name for the data dir
func getDataDir() string {
	var dir string
	switch {
	case runtime.GOOS == "windows":
		dir = os.Getenv("LocalAppData")
	case os.Getenv("XDG_DATA_HOME") != "":
		dir = os.Getenv("XDG_DATA_HOME")
	default: // Unix
		dir = os.Getenv("HOME")
		if dir == "" {
			return ""
		}
		dir = filepath.Join(dir, ".local", "share")
	}
	return filepath.Join(dir, "blisper")
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}

// same as the above, but without a value
func must_(err error) {
	if err != nil {
		panic(err)
	}
}

func contains[T comparable](arr []T, val T) bool {
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}

func dlModel(name string) string {
	validModels := []string{"tiny.en", "tiny", "base.en", "base", "small.en", "small", "medium.en", "medium", "large-v1", "large"}
	if !contains(validModels, name) {
		panic(fmt.Sprintf("invalid model name %s", name))
	}

	dataDir := getDataDir()
	if !pathExists(dataDir) {
		os.MkdirAll(dataDir, 0755)
	}

	outputFile := modelPath(name)
	if pathExists(outputFile) {
		return outputFile
	}

	out := must(os.Create(outputFile))
	defer out.Close()

	// https://github.com/ggerganov/whisper.cpp/blob/72deb41eb26300f71c50febe29db8ffcce09256c/models/download-ggml-model.sh#L9
	src := "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml"
	uri := fmt.Sprintf("%s-%s.bin", src, name)
	slog.Info("downloading file", "uri", uri)
	resp := must(http.Get(uri))
	defer resp.Body.Close()

	must(io.Copy(out, resp.Body))
	slog.Info("download complete")
	return outputFile
}

func modelPath(modelName string) string {
	name := fmt.Sprintf("ggml-%s.bin", modelName)
	return filepath.Join(getDataDir(), name)
}

type blisper struct {
	model   string
	infile  string
	outfile string
}

func run(args *blisper) error {
	modelPath := dlModel(args.model)

	// Load the model
	model := must(whisper.New(modelPath))
	defer model.Close()

	// Process samples
	context := must(model.NewContext())

	// modified from: https://github.com/ggerganov/whisper.cpp/blob/72deb41eb26300f71c50febe29db8ffcce09256c/bindings/go/examples/go-whisper/process.go#L31
	// TODO: command-line input file
	fh := must(os.Open(args.infile))
	defer fh.Close()

	// Decode the WAV file - load the full buffer
	// TODO: use ffmpeg bindings to generate proper wav files
	var data []float32 // Samples to process
	dec := wav.NewDecoder(fh)
	if buf, err := dec.FullPCMBuffer(); err != nil {
		return err
	} else if dec.SampleRate != whisper.SampleRate {
		return fmt.Errorf("unsupported sample rate: %d", dec.SampleRate)
	} else if dec.NumChans != 1 {
		return fmt.Errorf("unsupported number of channels: %d", dec.NumChans)
	} else {
		data = buf.AsFloat32Buffer().Data
	}

	context.ResetTimings()
	must_(context.Process(data, nil, nil))

	outf := must(os.Create(args.outfile))
	defer outf.Close()

	// Print out the results
	for {
		segment, err := context.NextSegment()
		if err != nil {
			break
		}
		fmt.Fprintf(outf, "[%6s->%6s] %s\n", segment.Start, segment.End, segment.Text)
	}

	return nil
}

func usage() {
	fmt.Println(`Usage: blisper [OPTIONS] <input-audio> <output-transcript>

Use whisper.cpp to transcribe the <input-audio> file into <output-transcript>

OPTIONS

  -model: The size of model to use. Defaults to small

MODELS

  Valid models are: tiny.en, tiny, base.en, base, small.en, small, medium.en, medium, large-v1, large

  Blisper will automatically download a model if you do not already have it on your system
  `)
}

func main() {
	if len(os.Args) != 3 {
		usage()
		return
	}
	var (
		model = flag.String("model", "small", "the model to use")
		help  = flag.Bool("help", false, "print help")
	)
	if *help {
		usage()
		return
	}
	run(&blisper{
		model:   *model,
		infile:  os.Args[1],
		outfile: os.Args[2],
	})
}
