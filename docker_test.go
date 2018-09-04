package main

import (
	"log"
	"testing"

	"github.com/spf13/afero"
	"gopkg.in/src-d/go-billy.v4/memfs"
)

func TestNewDockerBuilder(t *testing.T) {
	_, err := NewDockerBuilder(&BuildMatrix{}, []byte(""), "", "")
	if err != nil {
		t.Error("Should check for empty matrix or content")
	}
}

func TestFolderFromGitURL(t *testing.T) {
	const expected = "repository"
	urls := []string{
		"http://github.com/Owner/Repository",
		"https://github.com/Owner/repository",
		"git@github.com:Owner/repository.git",
	}
	for _, url := range urls {
		f := folderFromGitURL(url)
		if f != expected {
			t.Errorf("Expected %s got %s.", expected, f)
		}
	}
}

func TestDockerBuilderAddFile(t *testing.T) {
	err := afero.WriteFile(fs, "/test.txt", []byte("Test content"), 0644)
	if err != nil {
		t.Error(err)
	}

	log.Printf("FS: %+v\n", fs)
	dockerBuilder := &DockerBuilder{
		context: NewContext(),
	}
	err = dockerBuilder.addFile("", "/test.txt")
	if err != nil {
		t.Error("Should add file to context", err)
	}
	// TODO: Check that file is part of the context
}

func TestNewContext(t *testing.T) {
	context := NewContext()
	if context == nil {
		t.Error("Should return a new context")
	}
}

func TestContextAddFile(t *testing.T) {
	context := NewContext()
	if err := context.AddFile("test.txt", []byte("testing...")); err != nil {
		t.Error(err, "Should add a file to context")
	}
	// TODO: Check that file is part of the context
}

func TestAddFilesystem(t *testing.T) {
	context := NewContext()
	fs := memfs.New()
	fs.Create("test.txt")
	err := context.AddFilesystem("", "", fs)
	if err != nil {
		t.Error(err, "Should add filesystem to Docker context")
	}
	// TODO: Check file is present
}

func TestClose(t *testing.T) {
	context := NewContext()
	err := context.Close()
	if err != nil {
		t.Error(err, "Should close Docker context")
	}
}

func TestBytes(t *testing.T) {
	context := NewContext()
	b := context.Bytes()
	if len(b) > 0 {
		t.Error("Should return an empty context")
	}
}
