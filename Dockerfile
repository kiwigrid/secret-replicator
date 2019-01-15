FROM golang:alpine AS build-env

RUN apk add --no-cache git mercurial

# install glide
RUN go get github.com/Masterminds/glide
# create a working directory
WORKDIR /go/src/github.com/kiwigrid/pull-secret-distributor
# add glide.yaml and glide.lock
ADD glide.yaml glide.yaml
ADD glide.lock glide.lock
# install packages
RUN glide install
# add source code
ADD . .
RUN go build

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /go/src/github.com/kiwigrid/pull-secret-distributor/pull-secret-distributor /app/
ENTRYPOINT ./pull-secret-distributor