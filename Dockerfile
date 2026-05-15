FROM golang:1.22-alpine AS build

ARG VERSION=dev
ARG COMMIT=dev
ARG DATE=N/A

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w \
      -X github.com/rplevka/j9s/cmd.version=${VERSION} \
      -X github.com/rplevka/j9s/cmd.commit=${COMMIT} \
      -X github.com/rplevka/j9s/cmd.date=${DATE}" \
    -o /out/j9s .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 j9s

COPY --from=build /out/j9s /usr/local/bin/j9s

USER j9s
WORKDIR /home/j9s

ENTRYPOINT ["/usr/local/bin/j9s"]
