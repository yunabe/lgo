FROM golang:1.9

RUN apt-get update && apt-get install -y libzmq3-dev python-pip && rm -rf /var/lib/apt/lists/*
RUN pip install --upgrade pip && pip install -U jupyter jupyterlab && jupyter serverextension enable --py jupyterlab --sys-prefix
RUN go get github.com/yunabe/lgo/cmd/lgo && go get -d github.com/yunabe/lgo/cmd/lgo-internal
# Packages used from example notebooks.
RUN go get github.com/nfnt/resize
ENV LGOPATH /lgo
RUN $GOPATH/src/github.com/yunabe/lgo/bin/install_kernel
RUN lgo install

ENTRYPOINT ["jupyter", "notebook", "--ip=0.0.0.0", "--allow-root"]
