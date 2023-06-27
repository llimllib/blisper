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
	ffmpeg "github.com/u2takey/ffmpeg-go"

	"github.com/llimllib/blisper/fakestdio"
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

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
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

// convertToWav will attempt to convert fh to a WAV file of the proper format
// for whisper.cpp with ffmpeg
func convertToWav(f string, verbose bool) *os.File {
	// TODO: maybe check first if the file is of the right format? Could use
	// the other wav library to do that? Sample output in ffmpeg-probe.json
	// p := must(ffmpeg.Probe(f))
	// fmt.Printf("%s\n", p)
	// os.Exit(1)

	// ffmpeg module writes to log and to stdout/err, so soak up its output
	fakeIO := must(fakestdio.New())

	outf := must(os.CreateTemp("", "blisper*.wav"))

	// $ ffmpeg -i f.webm -ar 16000 -ac 1 -c:a pcm_s16le test.wav
	args := ffmpeg.KwArgs{
		"ar":  "16000",
		"ac":  "1",
		"c:a": "pcm_s16le",
	}

	must_(ffmpeg.Input(f).Output(outf.Name(), args).OverWriteOutput().ErrorToStdOut().Run())

	// reset stdout and stderr, and get ffmpeg's output
	stdout, stderr, err := fakeIO.ReadAndRestore()
	if err != nil {
		panic(err)
	}

	if verbose {
		fmt.Printf("ffmpeg output:\n%s\n------\n%s", stdout, stderr)
		fmt.Printf("wrote wav file %s\n", outf.Name())
	}

	// TODO: Add a verbose mode, and output the wav file's name
	// fmt.Printf("wrote %s\n", outf.Name())

	return must(os.Open(outf.Name()))
}

type blisper struct {
	model   string
	infile  string
	outfile string
	verbose bool
}

func run(args *blisper) error {
	modelPath := dlModel(args.model)

	// redirect stderr and stdout to a file. Note that any panics that occur in
	// here will not be output.
	// We do this because whisper writes to stderr without any possibility of
	// configuring it. This _probably_ doesn't work on windows
	fakeIO := must(fakestdio.New())

	// it's annoying that whisper.cpp writes directly to stderr without any
	// possibility of config.
	// https://github.com/ggerganov/whisper.cpp/issues/504
	// Load the model.
	model := must(whisper.New(modelPath))
	defer model.Close()

	// restore stderr and stdout. This returns the stdout and stderr output
	// respectively, but for now we'll ignore it
	_, stderr, err := fakeIO.ReadAndRestore()
	if err != nil {
		panic(err)
	}

	if args.verbose {
		// whisper only outputs to stderr
		fmt.Printf("whisper output:\n%s", stderr)
	}

	fh := convertToWav(args.infile, args.verbose)

	// modified from: https://github.com/ggerganov/whisper.cpp/blob/72deb41eb26300f71c50febe29db8ffcce09256c/bindings/go/examples/go-whisper/process.go#L31
	// Decode the WAV file - load the full buffer
	// TODO: use ffmpeg bindings to generate proper wav files
	var data []float32 // Samples to process

	context := must(model.NewContext())

	// TODO can I use ffmpeg to do this?
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

  -model:       the size of the whisper model to use. Defaults to "small"
  -config:      print the config for this app
  -help, -h:    print this help
  -verbose, -v: print verbose output

MODELS

  Valid models are: tiny.en, tiny, base.en, base, small.en, small, medium.en, medium, large-v1, large

  Blisper will automatically download a model if you do not already have it on your system
  `)
}

func main() {
	var (
		config  = flag.Bool("config", false, "print config location")
		help    = flag.Bool("help", false, "print help")
		h       = flag.Bool("h", false, "print help")
		model   = flag.String("model", "small", "the model to use")
		verbose = flag.Bool("verbose", false, "verbose output")
		v       = flag.Bool("v", false, "verbose output")
	)

	flag.Parse()

	if *help || *h {
		usage()
		return
	}

	if *config {
		fmt.Printf("Model dir: %s\n", getDataDir())
		return
	}

	// args must be <program name> [OPTIONS] <infile> <outfile>
	if len(os.Args) < 3 {
		usage()
		return
	}

	run(&blisper{
		model:   *model,
		infile:  os.Args[len(os.Args)-2],
		outfile: os.Args[len(os.Args)-1],
		verbose: *verbose || *v,
	})
}
