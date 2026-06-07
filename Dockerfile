FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/manager ./cmd/manager

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /out/manager /manager

EXPOSE 8081
EXPOSE 8082

USER nonroot:nonroot

ENTRYPOINT ["/manager"]
