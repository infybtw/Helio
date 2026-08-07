FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -trimpath -ldflags="-s -w" -o /bin/tg_bot .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /bin/tg_bot /tg_bot

EXPOSE 8080

ENTRYPOINT ["/tg_bot"]
