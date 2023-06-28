list the symbols in the file:

`nm whisper/libwhisper.so`

(on linux apparently it's `nm -D`)

[this SO thread](https://stackoverflow.com/questions/4514745/how-do-i-view-the-list-of-functions-a-linux-shared-library-is-exporting)

suggests that `objdump` should work too, but I get:

```console
$ objdump -f whisper/libwhisper.so
whisper/libwhisper.so:	file format mach-o arm64
/Library/Developer/CommandLineTools/usr/bin/objdump: error: 'whisper/libwhisper.so': Invalid/Unsupported object file format
```

which I truly don't understand
