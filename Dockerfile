FROM golang:latest AS build

RUN apt update && \
  apt install curl unzip -y && \
  curl -fsSL https://bun.sh/install | bash

WORKDIR /work

ADD ./frontend /work/frontend

COPY go.mod /work/go.mod
COPY go.sum /work/go.sum
RUN go mod download

ADD . /work
RUN cd frontend && \
  $HOME/.bun/bin/bun install && \
  $HOME/.bun/bin/bun run build && \
  cd .. && \
  CGO_ENABLED=0 go build -o main.exe -ldflags "-s -w" .

FROM ubuntu:latest

RUN apt update && apt upgrade -y
RUN apt install curl unzip -y && \
  curl -fsSL https://bun.sh/install | bash

RUN $HOME/.bun/bin/bunx playwright install-deps chromium

RUN rm -rf ~/.bun && \
    apt clean

WORKDIR /
COPY --from=build /work/main.exe /main.exe
COPY ./config.example.jsonnet /config.jsonnet

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT [ "/main.exe" ]
