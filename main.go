package main

import (
	"context"
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

	"github.com/asticode/go-astisub"
	"github.com/go-audio/wav"
	progressbar "github.com/schollz/progressbar/v3"

	whisper "github.com/llimllib/blisper/whisper"
)

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

	// create a context that will cancel on interrupt. defer'ing stop
	// guarantees that when the function exits nothing will be listening
	// for the signal any longer
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// https://github.com/ggerganov/whisper.cpp/blob/72deb41eb26300f71c50febe29db8ffcce09256c/models/download-ggml-model.sh#L9
	src := "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml"
	uri := fmt.Sprintf("%s-%s.bin", src, name)
	req := must(http.NewRequestWithContext(ctx, "GET", uri, nil))
	resp := must(http.DefaultClient.Do(req))
	defer resp.Body.Close()

	// download to a `<filename>.part` file until the download is successfully complete
	inProgressDownloadName := outputFile + ".part"
	out := must(os.Create(inProgressDownloadName))
	defer os.Remove(inProgressDownloadName)

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		fmt.Sprintf("downloading %s model", yellow(name)),
	)

	// Check for context.canceled, so that we don't output an unsightly error if
	// a user cancels the program. If it's any other error, handle as normal
	_, err := io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		if err != context.Canceled {
			fmt.Println(err)
			panic(err)
		} else {
			os.Exit(1)
		}
	}

	// Download complete. rename the <filename>.part file -> <filename>
	must_(os.Rename(inProgressDownloadName, outputFile))

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
	} else if dec.SampleRate != whisper.SAMPLE_RATE {
		return nil, fmt.Errorf("unsupported sample rate: %d", dec.SampleRate)
	} else if dec.NumChans != 1 {
		return nil, fmt.Errorf("unsupported number of channels: %d", dec.NumChans)
	}
	return buf.AsFloat32Buffer().Data, nil
}

func writeText(segments []whisper.Segment, outf *os.File) error {
	for _, segment := range segments {
		// TODO: output the durations in a consistent-width format to make it
		// easier to read
		_, err := fmt.Fprintf(outf, "[%s -> %s]  %s", segment.Start, segment.End, segment.Text)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeSubs(outFilepath string, segments []whisper.Segment, format string) error {
	outf := must(os.Create(outFilepath))
	defer outf.Close()

	// No need for astisub to write text
	if format == "txt" {
		return writeText(segments, outf)
	}

	i := 0
	subs := astisub.NewSubtitles()
	for _, segment := range segments {

		item := astisub.Item{
			StartAt: segment.Start,
			EndAt:   segment.End,
			Index:   i,
			Lines: []astisub.Line{{
				Items: []astisub.LineItem{
					{Text: segment.Text},
				},
			}},
		}
		subs.Items = append(subs.Items, &item)
		i += 1
	}

	switch format {
	case "srt":
		return subs.WriteToSRT(outf)
	case "ssa":
		return subs.WriteToSSA(outf)
	case "stl":
		return subs.WriteToSTL(outf)
	case "ttml":
		return subs.WriteToTTML(outf)
	case "vtt":
		return subs.WriteToWebVTT(outf)
	}

	return nil
}

func run(args args) {
	if !args.quiet {
		fmt.Printf("%s\n", yellow("loading model"))
	}
	modelPath := dlModel(args.model)

	if !args.quiet {
		fmt.Printf("%s\n", yellow("preparing audio"))
	}

	fh := must(os.Open(args.infile))
	// First just assume it's a properly-formatted wav file
	samples, err := readWav(fh)
	if err != nil {
		// if there was an error, try using ffmpeg to convert it to the proper
		// wav format. If _that_ errors, panic and quit
		if args.verbose {
			fmt.Println(err)
			fmt.Printf("%s\n", yellow("attempting to convert to wav with ffmpeg"))
		}
		samples = must(readWav(convertToWav(args.infile, args.verbose)))
	}

	if !args.quiet {
		fmt.Printf("%s\n", yellow("transcribing audio file"))
	}

	whisper := whisper.New(modelPath, !args.verbose)
	segments := must(whisper.Transcribe(samples))

	if !args.quiet {
		fmt.Printf("writing %s with format %s\n",
			yellow(args.outfile),
			yellow(args.format))
	}

	must_(writeSubs(args.outfile, segments, args.format))
}

type args struct {
	// Format re
	format  string
	infile  string
	model   string
	quiet   bool
	stream  bool
	outfile string
	verbose bool
}

func usage() {
	fmt.Println(`Usage: blisper [OPTIONS] <input-audio> <output-transcript>

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
  `)
}

func main() {
	modelDefault := "small"
	var (
		config  = flag.Bool("config", false, "print config location")
		format  = flag.String("format", "txt", "the output format")
		help    = flag.Bool("help", false, "print help")
		h       = flag.Bool("h", false, "print help")
		model   = flag.String("model", modelDefault, "the model to use")
		m       = flag.String("m", modelDefault, "the model to use")
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

	legalFormats := []string{"srt", "ssa", "stl", "ttml", "vtt", "txt"}
	if !contains(legalFormats, *format) {
		fmt.Printf("%s\n", red("Invalid format. Must be one of %v", legalFormats))
		os.Exit(1)
	}

	var _model string
	if *model != modelDefault {
		fmt.Printf("setting model to %v\n", model)
		_model = *model
	} else if *m != modelDefault {
		fmt.Printf("setting model to %v\n", m)
		_model = *m
	} else {
		_model = modelDefault
	}

	// args must be <infile> <outfile>
	if len(flag.Args()) != 2 {
		fmt.Printf("%s\n\n", red("Missing argument. Must have [<infile>, <outfile>], got %v", flag.Args()))
		usage()
		return
	}

	run(args{
		format:  *format,
		infile:  os.Args[len(os.Args)-2],
		model:   _model,
		outfile: os.Args[len(os.Args)-1],
		quiet:   *quiet || *q,
		stream:  *stream,
		verbose: *verbose || *v,
	})
}
