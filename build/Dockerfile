FROM docker.io/library/golang:1.20-alpine AS build
ARG entrypoint="./cmd/server"

WORKDIR /app
COPY . .

RUN go mod download
RUN go vet -v ./...
RUN go test -v ./...
RUN go build -mod=readonly -v -o server ${entrypoint}

#-------------------------------------------------------------------------------

FROM gcr.io/distroless/base
COPY --from=build /app/server /app/server
ENTRYPOINT ["/app/server"]
