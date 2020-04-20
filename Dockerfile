FROM golang:alpine as builder
RUN apk update && apk upgrade && apk add --no-cache git
ADD ./ ./gomatrix
RUN mkdir /build
WORKDIR ./gomatrix
ENV GOOS=linux GOARCH=amd64 CGO_ENABLED=0
RUN go build -a -ldflags="-w -s" -installsuffix cgo -o /build/gomatrix .

FROM scratch
MAINTAINER Geert-Johan Riemer <geertjohan@geertjohan.net>
COPY --from=builder /build/gomatrix /gomatrix
ENTRYPOINT ["/gomatrix"]
