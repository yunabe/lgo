# lgo
Interactive Go REPL and Jupyter Notebook kernel

# Features
- Write and execute Go interactively.
- Jupyter Notebook integration
- Full Go language spec support
- Code completion and inspection in Jupyter Notebooks
- Currently, lgo is only supported on Linux. But you can use lgo on Mac and Windows with virtual machines or Docker.

# Jupyter notebook examples
- [Example notebooks on Jupyter nbviewer](http://nbviewer.jupyter.org/github/yunabe/lgo/tree/master/examples/)
- [Example notebooks on GitHub](https://github.com/yunabe/lgo/blob/master/examples/basics.ipynb)

# Quick Start with Docker
1. Install [Docker](https://docs.docker.com/engine/installation/) and [Docker Compose](https://docs.docker.com/compose/).
2. Clone the respository and run the docker container with docker-compose.
```
> git clone https://github.com/yunabe/lgo.git
> cd lgo/docker/jupyter
> docker-compose up -d
```
3. Check the name of the container started with `docker-compose` (e.g. `jupyter_jupyter_1`).
4. Get the URL to open the Jupyter Notebook
```
> docker exec jupyter_jupyter_1 jupyter notebook list
Currently running servers:
http://0.0.0.0:8888/?token=50dfee7e328bf86e70c234a2f06021e1df63a19641c86676 :: /examples
```
5. Open the Jupyter Notebook server with the authentication token above.

# Install
## Prerequisites
- lgo is supported only on Linux at this moment. On Windows or Mac OS, use virtual machines or dockers.
- [Install Go 1.8 or Go 1.9](https://golang.org/doc/install)
- Install [Jupyter Notebook](http://jupyter.readthedocs.io/en/latest/install.html)
- Install ZMQ (libczmq)
  - e.g. `sudo apt-get install libczmq-dev`

## Install
- `go get github.com/yunabe/lgo/cmd/lgo && go get -d github.com/yunabe/lgo/cmd/lgo-internal`
  - This installs `lgo` command into your `$GOPATH/bin`
- Set `LGOPATH` environment variable
- Run `lgo install`
  - This installs libraries in your `$GOPATH/src` to `LGOPATH` with specific compiler flags.
  - It takes long time to install libraries if there are a lot libraries in your `GOPATH`.
  - If `lgo install` fails, please check install log stored in `$LGOPATH/install.log`
  - If `lgo install` fails because some packages can not be built, use blacklist those packages with `-package_blacklist` flag.
- Install the kernel configuration to Jupyter Notebook
  - `$GOPATH/src/github.com/yunabe/lgo/bin/install_kernel`

## Usage: Jupyter Notebook
- Run `jupyter notebook` command to start Juyputer Notebook and select "Go (lgo)" from New Notebook menu.
- To show documents of packages, functions and variables in your code, move the cursor to the identifier you want to inspect and press `Shift-Tab`.
- Press `Tab` to complete code

<img width="400" height="225" src="doc/inspect.jpg">
<img width="400" height="225" src="doc/complete.jpg">

# Comparisons with similar projects
TBD

# Trouble shootings

## old export format no longer supported
### Symptom
Got error messages like:

```
could not import github.com/yunabe/mylib (/home/yunabe/local/gocode/pkg/linux_amd64/github.com/yunabe/mylib.a: import "github.com/yunabe/mylib": old export format no longer supported (recompile library))
```

### Reason and Solution
Some libraries installed in your `$GOPATH` are in the old format, which are built go1.6 or before.
Make sure all libraries under your `$GOPATH` are recompiled with your current go compiler.

```
cd $GOPATH/src; go install ./...
```
