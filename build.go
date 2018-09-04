package main

import (
	"bufio"
	"bytes"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	yaml "gopkg.in/yaml.v2"
)

var (
	fs afero.Fs
	// YAMLDelim is the yaml frontmatter delimiter used on top of Dockerfiles
	YAMLDelim = []byte("---")
)

func init() {
	fs = afero.NewOsFs()
}

const (
	// LineEnd is a default UNIX line ending
	LineEnd = '\n'

	ErrInvalidFile       = "Please pass a valid Dockerfile"
	ErrFileRead          = "Can't read file"
	ErrScanningFile      = "Error scanning file"
	ErrDecodeFrontMatter = "Error decode frontmatter"
	ErrReadParseFile     = "Can't read and parse file"
	ErrEmptyFrontMatter  = "Error no frontmatter content"
	ErrEmptyDockerfile   = "Error no Dockerfile content"
	ErrNewDockerBuilder  = "Error creating a new DockerBuilder"
	ErrDockerBuild       = "Error build Docker images"
)

type BuildMatrix struct {
	Files []string
	Git   map[string]string
}

// Builder is an application builder
type Builder struct {
	buildMatrix *BuildMatrix
	file        afero.File
	frontMatter bytes.Buffer
	dockerfile  bytes.Buffer
}

// NewBuilder returns a new builder
func NewBuilder(filepath string) (*Builder, error) {
	builder := &Builder{}
	if filepath == "" {
		builder.file = os.Stdin
	} else {
		fd, err := fs.Stat(filepath)
		if err != nil || fd.IsDir() {
			return nil, errors.Wrap(err, ErrInvalidFile)
		}
		builder.file, err = fs.Open(filepath)
		if err != nil {
			return nil, errors.Wrap(err, ErrFileRead)
		}
		defer builder.file.Close()
	}
	err := builder.parseFile()
	if err != nil {
		return nil, errors.Wrap(err, ErrReadParseFile)
	}
	err = yaml.Unmarshal(builder.frontMatter.Bytes(), &builder.buildMatrix)
	if err != nil {
		return nil, errors.Wrap(err, ErrDecodeFrontMatter)
	}
	return builder, nil
}

// parseFile reads the file content to extract the frontmatter
// and the Dockerfile content
func (b *Builder) parseFile() error {
	scanner := bufio.NewScanner(b.file)
	var writeFrontMatter bool
	for scanner.Scan() {
		line := scanner.Bytes()
		if bytes.Equal(line, YAMLDelim) {
			if !writeFrontMatter {
				writeFrontMatter = true
			} else {
				writeFrontMatter = false
			}
		} else {
			if writeFrontMatter {
				b.frontMatter.Write(line)
				b.frontMatter.WriteRune(LineEnd)
			}
			if !writeFrontMatter {
				b.dockerfile.Write(line)
				b.dockerfile.WriteRune(LineEnd)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, ErrScanningFile)
	}
	if b.frontMatter.Len() == 0 {
		return errors.New(ErrEmptyFrontMatter)
	}
	if b.dockerfile.Len() == 0 {
		return errors.New(ErrEmptyDockerfile)
	}
	return nil
}

// Build runs the build based on the Builder's frontmatter and Dockerfile
// informations
func (b *Builder) Build(tag, branchOverride string) error {
	dockerBuilder, err := NewDockerBuilder(b.buildMatrix, b.dockerfile.Bytes(), tag, branchOverride)
	if err != nil {
		return errors.Wrap(err, ErrNewDockerBuilder)
	}
	err = dockerBuilder.Build()
	if err != nil {
		return errors.Wrap(err, ErrDockerBuild)
	}
	return nil
}
