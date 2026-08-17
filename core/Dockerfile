FROM golang:1.24-alpine AS build
WORKDIR /src
ARG SOURCE_COMMIT=
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
	-ldflags="-s -w -X main.version=${SOURCE_COMMIT}" \
	-o /out/autosecrets-core ./cmd/autosecrets-core

FROM alpine:3.20
RUN addgroup -S autosecrets && adduser -S -G autosecrets autosecrets
COPY --from=build /out/autosecrets-core /usr/local/bin/autosecrets-core
USER autosecrets
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/autosecrets-core"]
