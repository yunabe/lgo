FROM golang:1.9.2

RUN apt-get update && apt-get install -y libczmq-dev python-pip && rm -rf /var/lib/apt/lists/*
RUN pip install --upgrade pip && pip install -U jupyter
RUN go get github.com/yunabe/lgo/cmd/lgo && go get -d github.com/yunabe/lgo/cmd/lgo-internal
ENV LGOPATH /lgo
RUN $GOPATH/src/github.com/yunabe/lgo/bin/install_kernel
RUN lgo install
