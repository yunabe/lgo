FROM golang:1.9

# Install Jupyter Notebook
RUN apt-get update && apt-get install -y libzmq3-dev python-pip && rm -rf /var/lib/apt/lists/*
RUN pip install --upgrade pip && pip install -U jupyter jupyterlab && jupyter serverextension enable --py jupyterlab --sys-prefix

ENV LGOPATH /lgo
RUN mkdir -p $LGOPATH

# Add a non-root user with uid:1000 to follow the convention of mybinder to use this image from mybinder.org.
# https://mybinder.readthedocs.io/en/latest/dockerfile.html
ARG NB_USER=gopher
ARG NB_UID=1000
ENV HOME /home/${NB_USER}
RUN adduser --disabled-password \
    --gecos "Default user" \
    --uid ${NB_UID} \
    --home ${HOME} \
    ${NB_USER}
RUN chown -R ${NB_USER}:${NB_USER} ${HOME} $GOPATH $LGOPATH
USER ${NB_USER}
WORKDIR ${HOME}

# Fetch lgo repository
RUN go get github.com/yunabe/lgo/cmd/lgo && go get -d github.com/yunabe/lgo/cmd/lgo-internal

# Install packages used from example notebooks.
RUN go get -u github.com/nfnt/resize gonum.org/v1/gonum/... gonum.org/v1/plot/...

# Install lgo
RUN lgo install
RUN $GOPATH/src/github.com/yunabe/lgo/bin/install_kernel

# Notes:
# 1. Do not use ENTRYPOINT because mybinder need to run a custom command.
# 2. To use JupyterLab, replace "notebook" with "lab".
# 3. Set --allow-root in case you want to run jupyter as root.
CMD ["jupyter", "notebook", "--ip=0.0.0.0"]
