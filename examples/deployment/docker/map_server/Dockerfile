FROM golang:1.14 as build

WORKDIR /trillian

ARG GOFLAGS=""
ENV GOFLAGS=$GOFLAGS
ENV GO111MODULE=on

# Download dependencies first - this should be cacheable.
COPY go.mod go.sum ./
RUN go mod download

# Now add the local Trillian repo, which typically isn't cacheable.
COPY . .

# Build the server.
RUN go get ./cmd/trillian_map_server
# Run the licensing tool and save licenses, copyright notices, etc.
RUN go run github.com/google/go-licenses save ./cmd/trillian_map_server --save_path /THIRD_PARTY_NOTICES

RUN go get github.com/go-delve/delve/cmd/dlv

# Make a minimal image.
FROM gcr.io/distroless/base

COPY --from=build /go/bin/trillian_map_server /
COPY --from=build /go/bin/dlv /
COPY --from=build /THIRD_PARTY_NOTICES /THIRD_PARTY_NOTICES

CMD ["dlv", "debug", "--headless", "--listen=:9999", "--log"]

ENTRYPOINT ["/trillian_map_server"]
