FROM golang:1.18.1 AS baseline

ENV PYTHONUNBUFFERED=1

# If switching to a raw Linux distro image (e. g. Alpine),
# this must be part of the installation pipeline.
# RUN wget https://go.dev/dl/go1.18.1.linux-amd64.tar.gz
# RUN rm -rf /usr/local/go && tar -C /usr/local -xzf go1.18.1.linux-amd64.tar.gz
RUN apt update
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

WORKDIR /home/goat

FROM baseline AS go-run-bm
COPY --from=go-bm-prep  /home/GOAT /home/GOAT

WORKDIR /home/GOAT
ENTRYPOINT "bash"