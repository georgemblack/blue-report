FROM golang:1.23 AS build

WORKDIR /app
COPY . .

ENV GOOS=linux
ENV GOARCH=arm64
ENV CGO_ENABLED=0
RUN go build -o generate cmd/generate/main.go
RUN go build -o intake cmd/intake/main.go

FROM alpine

# Install timezone data used by Go
RUN apk add tzdata

COPY --from=build /app/generate /generate
COPY --from=build /app/intake /intake

CMD ["/intake"]