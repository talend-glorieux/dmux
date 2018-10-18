package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	billy "gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

const (
	Dockerfile       = "Dockerfile"
	dockerAPIVersion = "1.24"

	// Errors
	ErrAddDockerfile                     = "Error adding Dockerfile"
	ErrMappingFilesystem                 = "Error mapping filesystem"
	ErrClosingContext                    = "Error adding context"
	ErrGitWorktree                       = "Error Git worktreen"
	ErrGitCheckout                       = "Error checkout Git"
	ErrGetRepository                     = "Error getting repository at reference"
	ErrFileSystemEncodingToDockerContext = "Error encoding file system into Docker context"
	ErrReadFile                          = "Error reading file %s"
	// ErrBuild are triggered when Docker can't build an image
	ErrBuild = "Error building Docker image"
	// ErrListImage is triggered if we can't list images
	ErrListImage   = "Error listing images"
	ErrTagImage    = "Error tagging image"
	ErrRemoveImage = "Error removing image"
	// ErrCreateClient are triggered when we can't connect to the Docker deamon
	ErrCreateClient = "Error creating a new Docker client"
	// ErrTarHeaderWrite are triggered when we can't write context header
	ErrTarHeaderWrite = "Error writing file tar header: %v"
	// ErrTarFileWrite are triggered when we can't write file content to context
	ErrTarFileWrite = "Error writing tar file"
	// ErrTarClose are triggered when we can't close the context
	ErrTarClose = "Error closing tar buffer"
	// ErrOutputStream are triggered when we can't display docker output stream
	ErrOutputStream = "Error displaying Docker output stream"
	ErrFileStat     = "Error file stat %s"
	ErrReadDir      = "Error reading director %s"
	ErrAddFile      = "Error adding file"
	ErrUserDir      = "Error getting user directory"
)

// GitStore handles git file system and reference storage
type GitStore struct {
	fs     billy.Filesystem
	storer *memory.Storage
}

// DockerBuilder is a docker container builder
type DockerBuilder struct {
	tags []string
	sync.RWMutex
	gitStore map[string]*GitStore
	context  *Context
}

// NewDockerBuilder returns a build ready DockerBuilder
// It will clone then checkout the repositories specified on the Dockerfile
// FrontMatter then transform the produced Git file systems into Docker contexts
func NewDockerBuilder(matrix *BuildMatrix, dockerfile []byte, tag, branchOverride string) (*DockerBuilder, error) {
	dockerBuilder := &DockerBuilder{
		tags:     []string{tag},
		gitStore: make(map[string]*GitStore),
	}
	var wg sync.WaitGroup
	for repository, reference := range matrix.Git {
		wg.Add(1)
		dockerBuilder.Lock()
		dockerBuilder.gitStore[repository] = &GitStore{
			fs:     memfs.New(),
			storer: memory.NewStorage(),
		}
		dockerBuilder.Unlock()
		var ref = reference
		if branchOverride != "" {
			ref = branchOverride
		}
		go func(repository, reference string) error {
			err := dockerBuilder.getRepositoryAtReference(repository, reference)
			if err != nil {
				wg.Done()
				fmt.Println("Error", err)
				return errors.Wrap(err, ErrGetRepository)
			}
			wg.Done()
			return nil
		}(repository, ref)
	}
	wg.Wait()
	err := dockerBuilder.getDockerContextFromFilesystem(dockerfile, matrix.Files)
	if err != nil {
		return nil, errors.Wrap(err, ErrFileSystemEncodingToDockerContext)
	}
	return dockerBuilder, nil
}

// Build run the docker build described in the DockerBuilder
func (db *DockerBuilder) Build() error {
	fmt.Println("Sending Docker context")
	dockerClient, err := client.NewClient(client.DefaultDockerHost, dockerAPIVersion, nil, nil)
	if err != nil {
		return errors.Wrap(err, ErrCreateClient)
	}
	resp, err := dockerClient.ImageBuild(context.Background(), bytes.NewReader(db.context.Bytes()), types.ImageBuildOptions{
		ForceRemove: true,
	})
	if err != nil {
		return errors.Wrap(err, ErrBuild)
	}
	defer resp.Body.Close()
	err = jsonmessage.DisplayJSONMessagesToStream(
		resp.Body,
		NewOutStream(os.Stdout),
		nil,
	)
	if err != nil {
		return errors.Wrap(err, ErrOutputStream)
	}
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "build_tag")
	filterArgs.Add("dangling", "true")
	images, err := dockerClient.ImageList(context.Background(), types.ImageListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return errors.Wrap(err, ErrListImage)
	}
	for _, image := range images {
		tag := image.Labels["build_tag"]
		if tag != "" {
			fmt.Println("Tagging", image.ID, image.Labels["build_tag"])
			err := dockerClient.ImageTag(context.Background(), image.ID, image.Labels["build_tag"])
			if err != nil {
				return errors.Wrap(err, ErrTagImage)
			}
		}
	}
	// Once tagged we need to make sure we remove the old dangling images so they are not picked up on next run
	deletableImages, err := dockerClient.ImageList(context.Background(), types.ImageListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return errors.Wrap(err, ErrListImage)
	}
	for _, image := range deletableImages {
		resp, err := dockerClient.ImageRemove(context.Background(), image.ID, types.ImageRemoveOptions{
			Force:         true,
			PruneChildren: true,
		})
		if err != nil {
			return errors.Wrap(err, ErrRemoveImage)
		}
		fmt.Printf("Deleting: %v\n", resp)
	}

	return nil
}

func folderFromGitURL(url string) string {
	path := strings.Split(url, "/")
	return strings.ToLower(strings.TrimSuffix(path[len(path)-1], ".git"))
}

func (db *DockerBuilder) addFile(dir, filePath string) error {
	content, err := afero.ReadFile(fs, filePath)
	if err != nil {
		return errors.Wrapf(err, ErrReadFile, filePath)
	}
	err = db.context.AddFile(filepath.Join(dir, filepath.Base(filePath)), content)
	if err != nil {
		return errors.Wrap(err, ErrAddDockerfile)
	}
	return nil
}

func (db *DockerBuilder) getDockerContextFromFilesystem(dockerfile []byte, files []string) error {
	fmt.Println("Building Docker context")
	db.context = NewContext()
	if err := db.context.AddFile(Dockerfile, dockerfile); err != nil {
		return errors.Wrap(err, ErrAddDockerfile)
	}

	for _, filePath := range files {
		usr, err := user.Current()
		if filePath[:2] == "~/" {
			filePath = filepath.Join(usr.HomeDir, filePath[2:])
		}
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return errors.Wrapf(err, ErrFileStat, filePath)
		}
		if fileInfo.IsDir() {
			filesInfo, err := ioutil.ReadDir(filePath)
			if err != nil {
				return errors.Wrapf(err, ErrReadDir, filePath)
			}
			if err != nil {
				return errors.Wrapf(err, ErrUserDir)
			}
			for _, f := range filesInfo {
				err := db.addFile(filepath.Base(filePath), filepath.Join(filePath, f.Name()))
				if err != nil {
					return errors.Wrap(err, ErrAddFile)
				}
			}
		} else {
			if err := db.addFile("", filePath); err != nil {
				return errors.Wrap(err, ErrAddFile)
			}
		}
	}

	for key, store := range db.gitStore {
		if err := db.context.AddFilesystem(folderFromGitURL(key), ".", store.fs); err != nil {
			return errors.Wrap(err, ErrMappingFilesystem)
		}
	}

	if err := db.context.Close(); err != nil {
		return errors.Wrap(err, ErrClosingContext)
	}

	return nil
}

func (db *DockerBuilder) getRepositoryAtReference(repository, reference string) error {
	// TODO: Cache the repository to avoid refetching every time
	fmt.Printf("Cloning %s at %s.\n", repository, reference)
	db.RLock()
	storer := db.gitStore[repository].storer
	fs := db.gitStore[repository].fs
	db.RUnlock()
	_, err := git.Clone(
		storer,
		fs,
		&git.CloneOptions{
			URL:           repository,
			Progress:      os.Stdout,
			ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", reference)),
			SingleBranch:  true,
			Depth:         1,
		},
	)
	if err != nil {
		return errors.Wrap(err, "Error cloning Git repository")
	}
	return nil
}

// Context is a Docker context as a tar archive
type Context struct {
	buf *bytes.Buffer
	tw  *tar.Writer
}

// NewContext returns a new context
func NewContext() *Context {
	buf := new(bytes.Buffer)
	return &Context{
		buf: buf,
		tw:  tar.NewWriter(buf),
	}
}

// AddFile adds a file the Docker context
func (c *Context) AddFile(name string, content []byte) error {
	header := &tar.Header{
		Name: name,
		Mode: 0600,
		Size: int64(len(content)),
	}
	if err := c.tw.WriteHeader(header); err != nil {
		return errors.Wrapf(err, ErrTarHeaderWrite, header)
	}
	if _, err := c.tw.Write(content); err != nil {
		return errors.Wrap(err, ErrTarFileWrite)
	}
	return nil
}

// AddFilesystem map a file system to a tar archive
func (c *Context) AddFilesystem(prefix, path string, fs billy.Filesystem) error {
	info, err := fs.Stat(path)
	if err != nil {
		return errors.Wrap(err, "Can't stat")
	}
	header, err := tar.FileInfoHeader(info, path)
	if err != nil {
		return errors.Wrap(err, "Can't get header from fileinfo")
	}
	header.Name = filepath.Join(prefix, path)
	err = c.tw.WriteHeader(header)
	if err != nil {
		return errors.Wrap(err, "Can't write header")
	}
	if info.IsDir() {
		files, err := fs.ReadDir(path)
		if err != nil {
			return errors.Wrap(err, "Can't read dir")
		}

		for _, f := range files {
			if err = c.AddFilesystem(prefix, filepath.Join(path, f.Name()), fs); err != nil {
				return errors.Wrapf(err, "Can't tar %s", filepath.Join(path, f.Name()))
			}
		}
		return nil
	}

	if header.Typeflag == tar.TypeReg {
		file, err := fs.Open(path)
		if err != nil {
			return errors.Wrap(err, "Can't open file")
		}

		written, err := io.CopyN(c.tw, file, info.Size())
		if err != nil && err != io.EOF && written != info.Size() {
			return errors.Wrap(err, "Can't copy file content")
		}
		if err := file.Close(); err != nil {
			return errors.Wrap(err, "Can't close file")
		}
	}

	return nil
}

// Close closes the Docker context
func (c *Context) Close() error {
	if err := c.tw.Close(); err != nil {
		return errors.Wrap(err, ErrTarClose)
	}
	return nil
}

// Bytes returns the representation of the Docker context
func (c *Context) Bytes() []byte {
	return c.buf.Bytes()
}

// OutStream is a Docker output stream
type OutStream struct {
	out        io.Writer
	fd         uintptr
	isTerminal bool
	state      *term.State
}

// NewOutStream returns a new
// OutStream object from a Writer
func NewOutStream(out io.Writer) *OutStream {
	fd, isTerminal := term.GetFdInfo(out)
	return &OutStream{out: out, fd: fd, isTerminal: isTerminal}
}

func (o *OutStream) Write(p []byte) (int, error) {
	return o.out.Write(p)
}

// FD returns the file descriptor number for this stream
func (o *OutStream) FD() uintptr {
	return o.fd
}

// IsTerminal returns true if this stream is connected to a
// terminal
func (o *OutStream) IsTerminal() bool {
	return o.isTerminal
}
