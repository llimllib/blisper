package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/go-audio/wav"

	"github.com/llimllib/blisper/fakestdio"
)

var (
	YELLOW = "\033[33m"
	RESET  = "\033[0m"
)

func yellow(s string) string {
	return fmt.Sprintf("%s%s%s", YELLOW, s, RESET)
}

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
		fmt.Println(err)
		panic(err)
	}
	return t
}

// same as the above, but without a value
func must_(err error) {
	if err != nil {
		fmt.Println(err)
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
	fmt.Printf("%s\n", yellow(fmt.Sprintf("downloading file %s", uri)))
	resp := must(http.Get(uri))
	defer resp.Body.Close()

	must(io.Copy(out, resp.Body))
	fmt.Printf("%s\n", yellow("download complete"))
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
	model   string
	infile  string
	outfile string
	verbose bool
}

// transcribe an audio file to something srt-ish (format to come later)
// modified from: https://github.com/ggerganov/whisper.cpp/blob/72deb41eb26300f71c50febe29db8ffcce09256c/bindings/go/examples/go-whisper/process.go#L31
func (b *blisper) run() error {
	modelPath := dlModel(b.model)

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

	if b.verbose {
		// whisper only outputs to stderr
		fmt.Printf("whisper output:\n%s", stderr)
	}

	fh := must(os.Open(b.infile))
	// First just assume it's a properly-formatted wav file
	data, err := readWav(fh)
	if err != nil {
		if b.verbose {
			fmt.Println(err)
			fmt.Printf("%s\n", yellow("attempting to convert to wav with ffmpeg"))
		}
		// if there was an error, try using ffmpeg to convert it to the proper
		// wav format. If _that_ errors, panic and quit
		data = must(readWav(convertToWav(b.infile, b.verbose)))
	}

	context := must(model.NewContext())

	context.ResetTimings()
	if b.verbose {
		fmt.Printf("%s\n", yellow("transcribing audio file"))
	}
	must_(context.Process(data, nil, nil))

	outf := must(os.Create(b.outfile))
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

	(&blisper{
		model:   *model,
		infile:  os.Args[len(os.Args)-2],
		outfile: os.Args[len(os.Args)-1],
		verbose: *verbose || *v,
	}).run()
}
