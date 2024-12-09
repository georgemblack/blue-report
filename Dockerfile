FROM golang:1.23 AS build

WORKDIR /app
COPY . .

ENV GOOS=linux
ENV GOARCH=arm64
ENV CGO_ENABLED=0
RUN go build -o aggregate cmd/aggregate/main.go
RUN go build -o intake cmd/intake/main.go

FROM alpine
COPY --from=build /app/aggregate /aggregate
COPY --from=build /app/intake /intake

CMD ["/intake"]