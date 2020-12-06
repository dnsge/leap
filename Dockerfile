# Build stage
FROM golang:1.15-alpine AS build

WORKDIR /go/src/github.com/dnsge/leap

ADD go.mod .
ADD go.sum .
RUN go mod download

ADD . .
RUN go build -o /go/bin/github.com/dnsge/leap /go/src/github.com/dnsge/leap/cmd/leap

# Final stage
FROM alpine

WORKDIR /app
COPY --from=build /go/bin/github.com/dnsge/leap /app/

ENTRYPOINT ["./leap"]
