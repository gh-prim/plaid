package library

import (
	"regexp"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// ignoreFileName is the per-folder exclusion file (R6).
const ignoreFileName = ".plaidignore"

// sampleMaxSize is the size under which a file named "sample" is skipped (R5).
const sampleMaxSize = 300 << 20

// ignoredDirs are folder names never scanned, lowercase (R5). Extras get
// their own handling later (backlog #23).
var ignoredDirs = map[string]bool{
	"@eadir": true, "#recycle": true, "$recycle.bin": true, "system volume information": true,
	"lost+found": true,
	"extras":     true, "featurettes": true, "behind the scenes": true, "deleted scenes": true,
	"trailers": true, "interviews": true, "shorts": true,
}

// partialExts are in-progress downloads, lowercase (R5).
var partialExts = map[string]bool{".part": true, ".!qb": true, ".crdownload": true, ".tmp": true}

var sampleWord = regexp.MustCompile(`(?i)(?:^|[^\pL\pN])sample(?:[^\pL\pN]|$)`)

// ignoredDir reports whether a folder name is skipped by the built-in rules.
func ignoredDir(name string) bool {
	return strings.HasPrefix(name, ".") || ignoredDirs[strings.ToLower(name)]
}

// ignoredFile reports whether a file is skipped by the built-in rules.
func ignoredFile(name string, size int64) bool {
	if strings.HasPrefix(name, ".") || partialExts[lowerExt(name)] {
		return true
	}
	return size < sampleMaxSize && sampleWord.MatchString(strings.TrimSuffix(name, lowerExt(name)))
}

// ignoreRule is one line of a .plaidignore file.
type ignoreRule struct {
	match  *gitignore.GitIgnore
	negate bool
}

// ignoreFile is a parsed .plaidignore. Lines are compiled one by one because
// the library can't tell "negated" from "not matched", which stacking needs.
type ignoreFile []ignoreRule

func parseIgnoreFile(content string) ignoreFile {
	var f ignoreFile
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		negate := strings.HasPrefix(line, "!")
		if negate {
			line = line[1:]
		}
		f = append(f, ignoreRule{match: gitignore.CompileIgnoreLines(line), negate: negate})
	}
	return f
}

// decide returns (ignored, matched) for a slash-separated path relative to
// the folder holding the file; directories end with "/". As in gitignore, the
// last matching line wins.
func (f ignoreFile) decide(rel string) (ignored, matched bool) {
	for _, r := range f {
		if r.match.MatchesPath(rel) {
			ignored, matched = !r.negate, true
		}
	}
	return ignored, matched
}

// ignoreStack holds the .plaidignore files found during a walk, keyed by the
// slash-separated folder path relative to the library folder ("" = root).
type ignoreStack map[string]ignoreFile

// ignored reports whether rel (slash-separated, relative to the library
// folder) is excluded. The deepest .plaidignore with a matching line decides,
// so a sub-folder can re-include what a parent excluded.
func (s ignoreStack) ignored(rel string, isDir bool) bool {
	if len(s) == 0 {
		return false
	}
	suffix := ""
	if isDir {
		suffix = "/"
	}
	dir := rel
	for {
		if i := strings.LastIndexByte(dir, '/'); i >= 0 {
			dir = dir[:i]
		} else if dir != "" {
			dir = ""
		} else {
			return false
		}
		if f, ok := s[dir]; ok {
			sub := rel
			if dir != "" {
				sub = rel[len(dir)+1:]
			}
			if ignored, matched := f.decide(sub + suffix); matched {
				return ignored
			}
		}
		if dir == "" {
			return false
		}
	}
}
