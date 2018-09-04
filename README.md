# dmux
[![Build Status](https://travis-ci.org/talend-glorieux/dmux.svg?branch=master)](https://travis-ci.org/talend-glorieux/dmux) [![Go Report Card](https://goreportcard.com/badge/github.com/talend-glorieux/dmux)](https://goreportcard.com/report/github.com/talend-glorieux/dmux)

Making multi-repository builds simple.

## Install

Get the [latest binaries](https://github.com/talend-glorieux/dmux/releases/latest).

## Usage

In order to use `dmux` you only need to define a map of your repositories 
and the branch to fetch from in a front matter on top of your Dockerfile.

```
---
git@github.com/owner/repository: master
---
FROM scratch
ADD repository .
```

You can add as many repositories as desired.

```
---
git@github.com/owner/repository-1: master
git@github.com/owner/repository-2: master
git@github.com/owner/repository-3: master
---
FROM scratch as repository-1
ADD repository-1 .
RUN build.sh
...
...
```

### Authentication
If you plan on using ssh to fetch your repositories you’ll need to make sure your ssh-agent is properly initialized.

```eval `ssh-agent` ```
`ssh-add`

## How does it work?

In order to build an image, Docker creates a context. A context is just a word
for a tar archive of the path passed to the build command. That context is then
sent to the Docker deamon in order to build the image.

`dmux` injects itself right before the context is created. It will create a
context of its own made of all the files, folders and git repository passed in
the front matter of the Dockerfile. After that it mimics how Docker works. 

## Limits

Since the repository content is stored in memory before being passed to the
docker context, it's a good idea to avoid repository with a lot of content.
