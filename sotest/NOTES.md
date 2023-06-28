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

---

To get verbose output from go build:

`go build -work -x main.go`

---

https://mt165.co.uk/blog/static-link-go/

> However we can statically link all this C code into the binary if we want. The docs imply this isn’t a 100% supported path, but it’s an option. This can give you the best of both worlds - libc’s advanced functionality, and a run-anywhere static binary.

> To do this, we tell Go’s toolchain to use an external linker rather than its own (it’ll go find one, usually GCC’s ld):

go build -ldflags "-linkmode 'external'"

> We then need to tell _that_ linker to produce a static binary, or rather we need to tell the go driver programme to tell its linker component to call out to ld and to tell that. So we end up with:

go build -ldflags "-linkmode 'external' -extldflags '-static'"

This command fails because of `-static`; it can't link "crt0". here's an SO thread on lots of reasons why `-static` won't work on mac:

https://stackoverflow.com/questions/3801011/ld-library-not-found-for-lcrt0-o-on-osx-10-6-with-gcc-clang-static-flag

> You’ll see a lot of stuff on the internet saying you only need the -extldflags bit, but that’s not been true for a while: Go started using its own linker by default a few years back.

Trying without `-static` gets a different error:

```
/Users/llimllib/.local/share/asdf/installs/golang/1.20.5/go/pkg/tool/darwin_arm64/link: running clang failed: exit status 1
Undefined symbols for architecture arm64:
  "xtld=clang", referenced from:
     implicit entry/start for main executable
ld: symbol(s) not found for architecture arm64
clang: error: linker command failed with exit code 1 (use -v to see invocation)
```
