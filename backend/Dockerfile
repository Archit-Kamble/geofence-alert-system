# Stage 1: compile the Go program. This image has the Go compiler (large).
FROM golang:1.22-alpine AS build
WORKDIR /src

# Copy the dependency list first. Docker caches this layer, so dependencies
# are downloaded again only when go.mod or go.sum changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 makes one self-contained binary with no C libraries needed.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /server .

# Stage 2: the image that actually runs. Only the binary is copied in,
# so the final image is a few megabytes instead of several hundred.
FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
