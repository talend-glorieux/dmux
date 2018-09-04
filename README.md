# dmux
[![Build Status](https://travis-ci.org/talend-glorieux/dmux.svg?branch=master)](https://travis-ci.org/talend-glorieux/dmux)

Making multi-repository builds simple.

## Install

Get the latest binaries.

## Usage

In order to use `dmake` you only need to define a map of your repositories 
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

## Limits

Since the repository content is stored in memory before being passed to the
docker context, it's a good idea to avoid repository with a lot of content.
