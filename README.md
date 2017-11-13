# lgo
Interactive Go REPL and Jupyter Notebook kernel

# Features
- Write and execute Go interactively.
- Jupyter Notebook integration
- Full Go language spec support
- Code completion and inspection in Jupyter Notebooks
- Currently, lgo is only supported on Linux. But you can use lgo on Mac and Windows with virtual machines or Docker.

<img src="https://drive.google.com/uc?export=view&id=12_7fHfKfdSy8SNXi0nsWznbsRgix9tGJ" width="400" height="366">

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

# Tips
## Cancellation
In lgo, you can interrupt execution by pressing "Stop" button (or pressing `I, I`) in Jupyter Notebook and pressing `Ctrl-C` in the interactive shell.

However, as you may know, Go does not allow you to cancel running goroutines with `Ctrl-C`. Go does not provide any API to cancel specific goroutines. The standard way to handle cancellation in Go today is to use [`context.Context`](https://golang.org/pkg/context/#Context) (Read [Go Concurrency Patterns: Context](https://blog.golang.org/context) if you are not familiar with context.Context in Go).

lgo creates a special context `_ctx` on every execution and `_ctx` is cancelled when the execution is cancelled. Please pass `_ctx` as a context.Context param of Go libraries you want to cancel. Here is [an example notebook of cancellation in lgo](http://nbviewer.jupyter.org/github/yunabe/lgo/blob/master/examples/interrupt.ipynb).

## Memory Management
In lgo, memory is managed by the garbage collector of Go. Memory not referenced from any variables or goroutines is collected and released automatically.

One caveat of memory management in lgo is that memory referenced from global variables are not released automatically when the global variables are shadowed by other global variables with the same names. For example, if you run the following code blocks, the 32MB RAM reserved in `[1]` is not released after executing `[2]` and `[3]` because

- `[2]` does not reset the value of `b` in `[1]`. It just defines another global variable `b` with the same name and shadows the reference to the first `b`.
- `[3]` resets `b` defined in `[2]`. The memory reserved in `[2]` will be released after `[3]`. But the memory reserved in `[1]` will not be released.

```
[1]
// Assign 32MB ram to b.
b := make([]byte, 1 << 25)
[2]
// This shadows the first b.
b := make([]byte, 1 << 24)
[3]
// This sets nil to the second b.
b = nil
```

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
