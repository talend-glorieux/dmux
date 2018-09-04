package main

import (
	"os"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

const testError = "Should return an error. Expected '%s'. Got '%s'.\n"

var validDockerfile = `---
files:
  - file.txt
git:
  repo: version
---
FROM scratch
`

func TestMain(m *testing.M) {
	fs = afero.NewMemMapFs()
	os.Exit(m.Run())
}

func TestNewBuilder(t *testing.T) {
	t.Run("Returns an error if file is invalid", func(t *testing.T) {
		if err := afero.WriteFile(fs, "/plop", []byte(""), 0644); err != nil {
			t.Error(err)
		}
		_, err := NewBuilder("plop")
		if err == nil || !strings.Contains(err.Error(), ErrInvalidFile) {
			t.Errorf(testError, ErrFileRead, err.Error())
		}
	})
	t.Run("Returns a Builder without errors", func(t *testing.T) {
		if err := afero.WriteFile(fs, "/Dockerfile", []byte(validDockerfile), 0644); err != nil {
			t.Error(err)
		}
		builder, err := NewBuilder("/Dockerfile")
		if err != nil {
			t.Error(err)
		}
		if builder == nil {
			t.Error("Should return a vaild builder")
		}
	})
}

// TODO: Improve to remove open close
func newBuilderFile(name, content string) (*Builder, error) {
	file, err := fs.Create(name)
	if err != nil {
		return nil, errors.New("Error creating file")
	}
	r, err := file.WriteString(content)
	if r == 0 || err != nil {
		return nil, errors.New("Error writing to file")
	}
	file.Close()
	file, err = fs.Open(name)
	if err != nil {
		return nil, errors.New("Error opening file")
	}
	return &Builder{
		file: file,
	}, nil
}

func TestParseFile(t *testing.T) {
	t.Run("Missing frontmatter content", func(t *testing.T) {
		builder, err := newBuilderFile("Dockerfile_frontmatter_content", "empty")
		if err != nil {
			t.Error(err)
		}
		err = builder.parseFile()
		if err == nil || !strings.Contains(err.Error(), ErrEmptyFrontMatter) {
			t.Errorf(testError, ErrFileRead, err.Error())
		}
	})
	t.Run("Missing Dockerfile content", func(t *testing.T) {
		builder, err := newBuilderFile("Dockerfile_dockerfile_content", "---\nrepo: version\n---\n")
		if err != nil {
			t.Error(err)
		}
		err = builder.parseFile()
		if err == nil || !strings.Contains(err.Error(), ErrEmptyDockerfile) {
			t.Errorf(testError, ErrFileRead, err.Error())
		}
	})
	t.Run("Parses the file", func(t *testing.T) {
		builder, err := newBuilderFile("Dockerfile_dockerfile_content", validDockerfile)
		if err != nil {
			t.Error(err)
		}
		err = builder.parseFile()
		if err != nil {
			t.Error(err)
		}
	})
}

func BenchmarkParseFile(b *testing.B) {
	builder, err := newBuilderFile("Dockerfile_dockerfile_content", validDockerfile)
	if err != nil {
		panic(err)
	}
	for i := 0; i < b.N; i++ {
		builder.parseFile()
	}
}

func TestBuild(t *testing.T) {
	t.Skip()
	builder, err := newBuilderFile("Dockerfile_dockerfile_content", validDockerfile)
	if err != nil {
		t.Error(err)
	}
	err = builder.Build("", "")
	if err != nil {
		t.Error(err)
	}
}
