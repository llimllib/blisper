package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go"
	"github.com/go-audio/wav"
	progressbar "github.com/schollz/progressbar/v3"
)

// TODO: for some reason, I absolutely cannot get the runtime to search the
// whisper directory using the `rpath` argument. I've tried many little options
// and I'm certain it's not a big change but it's killing me. I'm giving up for
// now and sticking `libwhisper.so` in the same dir as the binary
// - Q: how would I get `go build` to show me the linker flags it's using?
// - Q: is it a clang-vs-gcc thing?

var (
	YELLOW = "\033[33m"
	RED    = "\033[31m"
	PURPLE = "\033[35m"
	RESET  = "\033[0m"
)

// yellow returns a formatted string which will print to the console with a
// yellow color
func yellow(s string, a ...any) string {
	return YELLOW + fmt.Sprintf(s, a...) + RESET
}

// red returns a formatted string which will print to the console with a red
// color
func red(s string, a ...any) string {
	return RED + fmt.Sprintf(s, a...) + RESET
}

// purple returns a formatted string which will print to the console with a
// purple color
func purple(s string, a ...any) string {
	return PURPLE + fmt.Sprintf(s, a...) + RESET
}

func contains[T comparable](arr []T, val T) bool {
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}

// getDataDir returns the name for the dir where blisper should store the
// users' model files
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

// dlModel accepts a model name and will download the model file into the
// user's data directory (given by getDataDir) if it does not already exist. It
// returns the full path of the model file.
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

	// https://github.com/ggerganov/whisper.cpp/blob/72deb41eb26300f71c50febe29db8ffcce09256c/models/download-ggml-model.sh#L9
	src := "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml"
	uri := fmt.Sprintf("%s-%s.bin", src, name)
	req := must(http.NewRequest("GET", uri, nil))
	resp := must(http.DefaultClient.Do(req))
	defer resp.Body.Close()

	out := must(os.Create(outputFile))
	defer out.Close()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		fmt.Sprintf("downloading %s model", yellow(name)),
	)

	// handle a sigint while we're downloading
	done := make(chan bool)
	go func() {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
		select {
		case <-sigchan:
			// ignore errors here, we've been interrupted and we're on
			// best-effort at this point. Try to remove the partial download
			out.Close()
			os.Remove(outputFile)

			os.Exit(1)
		case <-done:
			// the download finished, remove the handler and continue
			signal.Stop(sigchan)
			return
		}
	}()

	must(io.Copy(io.MultiWriter(out, bar), resp.Body))

	// tell the interrupt handler we finished the download, it doesn't need to
	// run any longer
	done <- true

	fmt.Printf("%s\n", yellow("download complete"))
	return outputFile
}

// modelPath returns the full path to the file blisper expects the model for a
// given name to be in
func modelPath(modelName string) string {
	name := fmt.Sprintf("ggml-%s.bin", modelName)
	return filepath.Join(getDataDir(), name)
}

// convertToWav will attempt to convert fh to a WAV file of the proper format
// for whisper.cpp with ffmpeg
func convertToWav(f string, verbose bool) *os.File {
	outf := must(os.CreateTemp("", "blisper*.wav"))

	cmd := exec.Command("ffmpeg",
		"-y",    // overwrite without asking
		"-i", f, // input file
		"-ar", "16000", // 16kHz
		"-ac", "1", // mono
		"-c:a", "pcm_s16le", // audio codec
		outf.Name())
	stderr := must(cmd.StderrPipe())
	stdout := must(cmd.StdoutPipe())
	must_(cmd.Start())
	out := must(io.ReadAll(stdout))
	err := must(io.ReadAll(stderr))
	must_(cmd.Wait())

	if verbose {
		fmt.Println(cmd.String())
	}

	if verbose {
		fmt.Printf("ffmpeg output:\n%s\n------\n%s", out, err)
		fmt.Printf("wrote wav file %s\n", yellow(outf.Name()))
	}

	return must(os.Open(outf.Name()))
}

// pathExists returns true if the path exists, false otherwise
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// must accepts a value and an error, and returns the value if the error is
// nil. Otherwise, prints the error and panics
func must[T any](t T, err error) T {
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	return t
}

// must_ accepts an error, and prints and panics if present
func must_(err error) {
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}

// readWav reads a wav file and returns its decoded data or an error
func readWav(fh *os.File) ([]float32, error) {
	dec := wav.NewDecoder(fh)
	buf, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, err
	} else if dec.SampleRate != whisper.SampleRate {
		return nil, fmt.Errorf("unsupported sample rate: %d", dec.SampleRate)
	} else if dec.NumChans != 1 {
		return nil, fmt.Errorf("unsupported number of channels: %d", dec.NumChans)
	}
	return buf.AsFloat32Buffer().Data, nil
}

type blisper struct {
	format  string
	infile  string
	model   string
	quiet   bool
	stream  bool
	outfile string
	verbose bool
}

func (b *blisper) transcribe() {
	if !b.quiet {
		fmt.Printf("%s\n", yellow("loading model"))
	}
	modelPath := dlModel(b.model)
	// The function call we're going to try and reproduce is:
	// struct whisper_context * ctx = whisper_init_from_file(params.model.c_str());

	ctx := whisper.Whisper_init(modelPath)
	if ctx == nil {
		panic("uanble to init context")
	}

	if !b.quiet {
		fmt.Printf("%s\n", yellow("preparing audio"))
	}

	fh := must(os.Open(b.infile))
	// First just assume it's a properly-formatted wav file
	samples, err := readWav(fh)
	if err != nil {
		if b.verbose {
			fmt.Println(err)
			fmt.Printf("%s\n", yellow("attempting to convert to wav with ffmpeg"))
		}
		// if there was an error, try using ffmpeg to convert it to the proper
		// wav format. If _that_ errors, panic and quit
		samples = must(readWav(convertToWav(b.infile, b.verbose)))
	}

	if !b.quiet {
		fmt.Printf("%s\n", yellow("transcribing audio file"))
	}

	if b.verbose {
		fmt.Printf("%s\n", whisper.Whisper_print_system_info())
	}

	params := ctx.Whisper_full_default_params(whisper.SAMPLING_GREEDY)
	fmt.Printf("%#v\n", params)

	// The number of processors to utilize
	processors := 1

	// https://github.com/ggerganov/whisper.cpp/blob/72deb41e/bindings/go/whisper.go#L328
	// TODO: provide progress and segment callbacks
	must_(ctx.Whisper_full_parallel(params, samples, processors, nil, nil))

	// https://github.com/ggerganov/whisper.cpp/blob/72deb41e/bindings/go/pkg/whisper/context.go#L203
	// https://github.com/ggerganov/whisper.cpp/blob/72deb41e/examples/main/main.cpp#L309
	// https://github.com/ggerganov/whisper.cpp/blob/72deb41e/examples/main/main.cpp#L897
	n_segments := ctx.Whisper_full_n_segments()
	for i := 0; i < n_segments; i++ {
		text := ctx.Whisper_full_get_segment_text(i)
		fmt.Printf("%s\n", text)
	}
	fmt.Println()
}

func usage() {
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
  `)
}

func main() {
	var (
		config  = flag.Bool("config", false, "print config location")
		format  = flag.String("format", "srt", "the output format")
		help    = flag.Bool("help", false, "print help")
		h       = flag.Bool("h", false, "print help")
		model   = flag.String("model", "small", "the model to use")
		q       = flag.Bool("q", false, "silence all output")
		quiet   = flag.Bool("quiet", false, "silence all output")
		stream  = flag.Bool("stream", false, "stream output to stdout")
		v       = flag.Bool("v", false, "verbose output")
		verbose = flag.Bool("verbose", false, "verbose output")
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

	legalFormats := []string{"srt", "ssa", "stl", "ttml", "vtt"}
	if !contains(legalFormats, *format) {
		fmt.Printf("%s\n", red("Invalid format. Must be one of %v", legalFormats))
		os.Exit(1)
	}

	// args must be <program name> [OPTIONS] <infile> <outfile>
	if len(os.Args) < 3 {
		usage()
		return
	}

	(&blisper{
		format:  *format,
		infile:  os.Args[len(os.Args)-2],
		model:   *model,
		quiet:   *quiet || *q,
		stream:  *stream,
		outfile: os.Args[len(os.Args)-1],
		verbose: *verbose || *v,
	}).transcribe()
}
