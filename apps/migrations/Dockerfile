FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -trimpath -ldflags="-s -w" -o /bin/migrations .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /bin/migrations /migrations

ENTRYPOINT ["/migrations"]
CMD ["up"]
