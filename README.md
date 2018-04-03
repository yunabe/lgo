# lgo
[![Binder](https://mybinder.org/badge.svg)](https://mybinder.org/v2/gh/yunabe/lgo-binder/master?filepath=basics.ipynb)

Go (golang) Jupyter Notebook kernel and an interactive REPL

# Features
- Write and execute Go (golang) interactively like Python.
- Jupyter Notebook integration
- Full Go (golang) language spec support. 100% gc (go compiler) compatible.
- Code completion and inspection in Jupyter Notebooks
- Display images, HTML, JavaScript, SVG, etc...
- Currently, lgo is only supported on Linux. But you can use lgo on Mac and Windows with virtual machines or Docker.

<img src="https://drive.google.com/uc?export=view&id=12_7fHfKfdSy8SNXi0nsWznbsRgix9tGJ" width="400" height="366">

# Jupyter notebook examples
You can view example notebooks of lgo from
[Example notebooks on Jupyter nbviewer](https://nbviewer.jupyter.org/github/yunabe/lgo/blob/master/examples/basics.ipynb)

If you want to execute these notebooks, you can try these notebooks on your browser without installation from [![Binder](https://mybinder.org/badge.svg)](https://mybinder.org/v2/gh/yunabe/lgo-binder/master?filepath=basics.ipynb)

# Try lgo from your browser without installation
[![Binder](https://mybinder.org/badge.svg)](https://mybinder.org/v2/gh/yunabe/lgo-binder/master?filepath=basics.ipynb)

Thanks to [binder (mybinder.org)](https://mybinder.org/), you can try lgo on your browsers with temporary docker containers on binder.
Open your temporary Jupyter Notebook from the button above and enjoy lgo.

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
- [Install Go 1.9 or Go 1.8](https://golang.org/doc/install)
- Install [Jupyter Notebook](http://jupyter.readthedocs.io/en/latest/install.html)
- [Install ZMQ](http://zeromq.org/distro:debian)
  - e.g. `sudo apt-get install libzmq3-dev`

## Install
- `go get github.com/yunabe/lgo/cmd/lgo && go get -d github.com/yunabe/lgo/cmd/lgo-internal`
  - This installs `lgo` command into your `$GOPATH/bin`
- Set `LGOPATH` environment variable
  - `lgo install` will install binaries into the directory specified with `LGOPATH`.
  - You can use any empty directory with write permission as `LGOPATH`.
- Run `lgo install`
  - This installs std libraries and the internal lgo tool into `LGOPATH` with specific compiler flags.
  - If `lgo install` fails, please check install log stored in `$LGOPATH/install.log`
- (Optional) Run `lgo installpkg [packages]` to install third-party packages to `LGOPATH`
  - You can preinstall third-party packages into `LGOPATH`.
  - This step is optional. If packages are not preinstalled, lgo installs the packages on the fly.
  - But, installing packages is a heavy and slow process. I recommend you to preinstall packages
    which you will use in the future with high probability.
  - If `lgo installpkg` fails, please check the log stored in `$LGOPATH/installpkg.log`.
  - See [go's manual](https://golang.org/cmd/go/#hdr-Package_lists) about the format of `[packages]` args.
- Install the kernel configuration to Jupyter Notebook
  - `python $GOPATH/src/github.com/yunabe/lgo/bin/install_kernel`
  - Make sure to use the same version of `python` as you used to install `jupyter`. For example, use `python3` instead of `python` if you install `jupyter` with `pip3`.

## Usage: Jupyter Notebook
- Run `jupyter notebook` command to start Juyputer Notebook and select "Go (lgo)" from New Notebook menu.
- To show documents of packages, functions and variables in your code, move the cursor to the identifier you want to inspect and press `Shift-Tab`.
- Press `Tab` to complete code
- lgo works with [JupyterLab](https://github.com/jupyterlab/jupyterlab). To use lgo from JupyterLab, install JupyterLab and run `jupyter lab`.

<img width="400" height="225" src="doc/inspect.jpg">
<img width="400" height="225" src="doc/complete.jpg">

## Usage: REPL console
You can use lgo from command line with [Jupyter Console](https://github.com/jupyter/jupyter_console) or build-in REPL mode of lgo

### Jupyter Console (Recommended)
Run `jupyter console --kernel lgo`

```go
In [1]: a, b := 3, 4

In [2]: func sum(x, y int) int {
      :     return x + y
      :     }

In [3]: import "fmt"

In [4]: fmt.Sprintf("sum(%d, %d) = %d", a, b, sum(a, b))
sum(3, 4) = 7
```

### built-in REPL mode
Run `lgo run`

```go
$ lgo run
>>> a, b := 3, 4
>>> func sum(x, y int) int {
...     return x + y
...     }
>>> import "fmt"
>>> fmt.Sprintf("sum(%d, %d) = %d", a, b, sum(a, b))
sum(3, 4) = 7
```

# Tips
## go get and lgo
The packages you want to use in lgo must be prebuilt and installed into `$LGOPATH` by `lgo install` command.
Please make sure to run `lgo install` after you fetch a new package with `go get` command.

## Update go version
Please run `lgo install --clean` after you update `go` version.

`lgo install` installs prebuilt packages into `$LGOPATH`.
When you update `go` version, you need to reinstall these prebuilt packages with the newer `go`
because binary formats of prebuilt packages may change in the newer version of go.

## Display HTML and images
To display HTML and images in lgo, use [`_ctx.Display`](https://godoc.org/github.com/yunabe/lgo/core#LgoContext).
See [the example of `_ctx.Display`](http://nbviewer.jupyter.org/github/yunabe/lgo/blob/master/examples/basics.ipynb#Display) in an example notebook

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
## gore
[gore](https://github.com/motemen/gore), which was released in Feb 2015, is the most famous REPL implementation for Go as of Dec 2017. gore is a great tool to try out very short code snippets in REPL style.

But gore does not fit to data science or heavy data processing at all.
gore executes your inputs by concatinating all of your inputs,
wrapping it with `main` function and running it with `go run` command.
This means every time you input your code, gore executes all your inputs from the begining.
For example, if you are writing something like

1. Loads a very large CSV file as an input. It takes 1 min to load.
2. Analyzes the loaded data. For example, calculates max, min, avg, etc..

gore always runs the first step when you calculate something and you need to wait for 1 min every time.
This behavior is not acceptable for real data science works. Also, gore is not good at tyring code with side effects (even fmt.Println) because code snippets with side effects are executed repeatedly and repeatedly.
lgo chose a totally different approach to execute Go code interactively and does not have the same shortcoming.

gore is a CLI tool and it does not support Jupyter Notebook.

## gophernotes
||lgo|gophernotes|
|:---|:---|:---|
|Backend|gc (go compiler)|An unofficial interpreter|
|Full Go Language Specs|:heavy_check_mark:||
|100% gc compatible|:heavy_check_mark:||
|Type Safety|:heavy_check_mark:||
|Performance|Fast|Slow|
|Overhead|500ms|1ms|
|[Cancellation](https://github.com/yunabe/lgo/blob/master/README.md#cancellation)|:heavy_check_mark:||
|Code completion|:heavy_check_mark:||
|Code inspection|:heavy_check_mark:||
|[Display HTML and images](https://github.com/yunabe/lgo/blob/master/README.md#display-html-and-images)|:heavy_check_mark:||
|Windows, Mac|Use Docker or VM|Partial|
|License|BSD|LGPL|

[gophernotes](https://github.com/gopherdata/gophernotes) is the first Jupyter kernel for Go, released in Jan 2016.
Before [Sep 2017](https://github.com/gopherdata/gophernotes/commit/69792d8af799d6905e2c576164d1a189ac021784#diff-04c6e90faac2675aa89e2176d2eec7d8), it used the same technology gore uses to evaluate Go code. This means it did not fit to heavy data processing or data analysis at all.
From Sep 2017, gophernotes switched from `go run` approach to [gomacro](https://github.com/cosmos72/gomacro), one of unofficial golang interpreters by [cosmos72](https://github.com/cosmos72). This solved the problem gore has. Now, gophernotes is a great tool for data science in Go.

The shortcomings of using an unofficial interpreter are
- It does not support all Go language features. Especially, it does not support one of the most important Go feature, `interface`.
  As of go1.10, it is hard to support `interface` in an interpreter written in Go because of the lack of API in `reflect` package.
- Interpreters are generally slow.
- Type unsafe. At least, gomacro is not statically typed.
- Unofficial interpreters are not well-tested compared to the official gc (go compiler) tools.

The advantages of this approach are
- The overhead of code execution is small because it does not compile and link code.
- Windows/Mac partial support. lgo works only on Linux and you need to use VMs or Docker to run it on Windows/Mac.
  gophernotes (gomacro) works on Windows/Mac natively [if you do not need third-party packages](https://github.com/gopherdata/gophernotes#limitations).

These disadvantage and advantages are not something inevitable in interperters. But they are not easy to solve under the limited development resource.

Also, lgo kernel supports more rich features in Jupyter Notebook as of Dec 2017, including code completion, code inspection and images/HTML/JavaScript output supports.

# Troubleshooting

## Dead kernel
### Symptom
Got an error message like:

```
Kernel Restarting
The kernel appears to have died. It will restart automatically.
```

### Solutions
First, please confirm your code does not call [`os.Exit`](https://golang.org/pkg/os/#Exit) directly or indirectly.
In lgo, your code is executed in the processs of lgo kernel. If you evaluate `os.Exit` in lgo, it terminates the lgo kernel process and jupyter notebook server loses the connection with the kernel.
Thus, you must not evaluate `os.Exit` or functions that call it internally (e.g. `log.Fatal`) in lgo.

If `os.Exit` is not the reason of "Dead kernel", please check crash logs of the kernel.
If you run your notebook with `jupyter notebook` command in a terminal, the crash log should be there.
If you run your notebook in docker, attach the container's terminal with [`docker attach`](https://docs.docker.com/engine/reference/commandline/attach/) to view the logs.
If you can see the logs of `jupyter notebook`, you should see logs like

```
2018/03/01 20:30:45 lgo-internal failed: exit status 1
[I 22:34:00.500 NotebookApp] KernelRestarter: restarting kernel (1/5)
kernel abcd1234-5678-efghi-xxxx-777eeffcccbb restarted
```

and you can probably see helpful information before `lgo-internal failed` message.

## multiple roots
### Sympton
Got an error message like:

```
multiple roots $LGOPATH/pkg &
Failed to build a shared library of github.com/yunabe/lgo/sess7b..7d/exec1: exit status 1
```

### Solutions
This error occurs when the `go` command you are currently using is different from the `go` command you used to run `lgo install`.
For example, this happens if you update `go` from 1.9 to 1.10 but did not run `lgo install --clean` with the new `go` after the update.

If you encouter this issue, please double-check that you are using `go` which you used to run `lgo install` to install packages into `$LGOPATH`.

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
