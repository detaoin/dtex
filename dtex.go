// This is free and unencumbered software released into the public domain.
//
// Anyone is free to copy, modify, publish, use, compile, sell, or
// distribute this software, either in source code form or as a compiled
// binary, for any purpose, commercial or non-commercial, and by any
// means.
//
// In jurisdictions that recognize copyright laws, the author or authors
// of this software dedicate any and all copyright interest in the
// software to the public domain. We make this dedication for the benefit
// of the public at large and to the detriment of our heirs and
// successors. We intend this dedication to be an overt act of
// relinquishment in perpetuity of all present and future rights to this
// software under copyright law.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
// IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.
//
// For more information, please refer to <http://unlicense.org>

// dtex is a tex compiler wrapper tool.
package main

import (
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func Usage() {
	fmt.Fprintln(os.Stderr, "Usage: dtex [tex options] file.tex")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "will compile file.tex as many times as necessary: until all the generated")
	fmt.Fprintln(os.Stderr, "temporary files don't change anymore, with a maximum of 5 compilations.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage: dtex -clean")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "will remove all temporary files used by this program.")
	os.Exit(1)
}

// root directory where the TEX engine writes temporary files (.aux, ...)
var tmp = filepath.Join(os.TempDir(), "dtex")

func main() {
	SetLogOutput()
	log.Println("Using temporary root:", tmp)
	args := os.Args[1:]
	file := ParseArgs(args)
	tmpbase := GetTmp(file)
	args = append([]string{"-output-directory", filepath.Dir(tmpbase)}, args...)
	tex := GetTexEngine()

	log.Println("Computing initial hashes of", tmpbase)
	hashes := NewHashes(tmpbase)
	for try := 0; hashes.Changed() && try < 5; try++ {
		log.Println("Compile iteration", try)
		Compile(tex, args)
		log.Println("Updating hashes")
		hashes.Update()
	}
	if hashes.Changed() {
		fmt.Fprintln(os.Stderr, "Warning: 5 compilations were maybe insufficient")
	}
	if err := os.Rename(tmpbase+".pdf", file+".pdf"); err != nil {
		Err("Move resulting pdf into place: %v\n", err)
	}
}

func IsVerbose() bool { return os.Getenv("VERBOSE") != "" }

func Err(msg string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, a...)
	os.Exit(1)
}

func SetLogOutput() {
	log.SetOutput(ioutil.Discard)
	if IsVerbose() {
		log.SetOutput(os.Stderr)
	}
}

func ParseArgs(args []string) string {
	if len(args) == 1 && args[0] == "-clean" {
		log.Printf("rm -r %q\n", tmp)
		if err := os.RemoveAll(tmp); err != nil {
			Err("clean temporary files: %v\n", err)
		}
		os.Exit(0)
	}
	if len(args) < 1 {
		Usage()
	}
	for _, a := range args {
		if a == "-output-directory" {
			Err("\"-output-directory\" flag not allowed\n")
		}
	}
	file := args[len(args)-1]
	if filepath.Ext(file) == ".tex" {
		file = file[:len(file)-len(".tex")]
	}
	return file
}

func GetTmp(file string) string {
	base, err := filepath.Abs(file)
	if err != nil {
		log.Printf("absolute path(%q): %v\n", file, err)
		base = filepath.Base(file)
	}
	volume := filepath.VolumeName(base)
	base = base[len(volume):]
	tmpbase := filepath.Join(tmp, base)
	tmpdir := filepath.Dir(tmpbase)
	if err := os.MkdirAll(tmpdir, 0755); err != nil {
		Err("Create temporary directory (%v): %v\n", tmpdir, err)
	}
	return tmpbase
}

func GetTexEngine() string {
	tex := "pdflatex"
	if t := os.Getenv("TEX"); t != "" {
		tex = t
	}
	return tex
}

type Hashes struct {
	base string
	h    map[string]uint64
	mod  bool
}

func NewHashes(base string) *Hashes {
	h := &Hashes{base: base, h: map[string]uint64{}}
	h.Update()
	h.mod = true
	return h
}

func (h *Hashes) Update() {
	pat := filepath.Join(filepath.Dir(h.base), "*.*")
	files, err := filepath.Glob(pat)
	if err != nil {
		Err("bad filepath.Glob(%q): %v\n", pat, err)
	}
	h.mod = false
	for _, file := range files {
		ext := filepath.Ext(file)
		if ext == ".pdf" || ext == ".log" {
			continue
		}
		id := HashFile(file)
		log.Println("Hashing", file, "→", id)
		if id != h.h[file] {
			log.Println("  file changed")
			h.mod = true
		}
		h.h[file] = id
	}
}

func (h *Hashes) Changed() bool { return h.mod }

func HashFile(file string) uint64 {
	h := fnv.New64a()
	f, err := os.Open(file)
	if err != nil {
		Err("Open file (%v): %v\n", file, err)
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		Err("Read file (%v): %v\n", file, err)
	}
	return h.Sum64()
}

func Compile(tex string, args []string) {
	log.Printf("Running %v %q\n", tex, args)
	out, err := exec.Command(tex, args...).CombinedOutput()
	if err != nil {
		os.Stdout.Write(out)
		Err("Compilation error: %v\n", err)
	}
}
