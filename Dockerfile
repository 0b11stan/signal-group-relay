FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /signal2signal ./cmd/signal2signal

FROM scratch
COPY --from=build /signal2signal /signal2signal
USER 65534:65534
ENTRYPOINT ["/signal2signal"]
