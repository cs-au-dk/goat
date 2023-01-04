# FROM golang:1.16.4 AS gcatch-repo
# RUN git clone https://github.com/system-pclub/GCatch.git /repos/GCatch

# FROM golang:1.16.4 AS gcatch-baseline

# RUN git clone https://github.com/Z3Prover/z3 /repos/z3
# WORKDIR /repos/z3
# RUN python scripts/mk_make.py \ 
#   && cd build \
#   && make \
#   && make install

# COPY --from=gcatch-repo /repos/GCatch/GCatch /go/src/github.com/system-pclub/GCatch/GCatch

# RUN GO111MODULE=off go get golang.org/x/xerrors \
# && GO111MODULE=off go get golang.org/x/mod/semver \
# && GO111MODULE=off go get golang.org/x/sys/execabs

# RUN /go/src/github.com/system-pclub/GCatch/GCatch/install.sh

FROM golang:1.18.1 AS baseline

ARG REPO=""
ARG ACTION="./benchmarks/_interactive.sh"
ENV COM=$ACTION

ENV PYTHONUNBUFFERED=1

# If switching to a raw Linux distro image (e. g. Alpine),
# this must be part of the installation pipeline.
# RUN wget https://go.dev/dl/go1.18.1.linux-amd64.tar.gz
# RUN rm -rf /usr/local/go && tar -C /usr/local -xzf go1.18.1.linux-amd64.tar.gz
RUN apt update
# Not needed until visualization works
# RUN (echo "Y" && cat) | apt install xdot
RUN (echo "Y" && cat) | apt install python3-pip
RUN (echo "Y" && cat) | apt install rsync

RUN pip install --upgrade pip
RUN pip install setuptools
RUN pip install tqdm

FROM baseline AS go-baseline
RUN go install golang.org/x/tools/cmd/goimports@latest

FROM go-baseline AS goat-build
COPY . /home/GOAT
WORKDIR /home/GOAT

RUN go generate ./...

FROM baseline AS go-bm-prep
COPY --from=goat-build /home/GOAT /home/GOAT

WORKDIR /home/GOAT

RUN ./benchmarks/clone.sh $REPO
RUN ./benchmarks/apply_patches.py

FROM baseline AS go-run-bm
COPY --from=go-bm-prep  /home/GOAT /home/GOAT

WORKDIR /home/GOAT
ENTRYPOINT "bash" $COM
# RUN ./benchmarks/run.py --dir repo_run --repo $REPO --flags -psets gcatch