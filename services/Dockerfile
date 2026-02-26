FROM golang:1.26 AS build

WORKDIR /app
COPY . .

ENV GOOS=linux
ENV GOARCH=arm64
ENV CGO_ENABLED=0
RUN go build -o intake cmd/intake/main.go
RUN go build -o link_aggregation cmd/link_aggregation/main.go
RUN go build -o site_aggregation cmd/site_aggregation/main.go
RUN go build -o link_redirect cmd/link_redirect/main.go

FROM alpine

# Install timezone data used by Go
RUN apk add tzdata

COPY --from=build /app/intake /intake
COPY --from=build /app/link_aggregation /link_aggregation
COPY --from=build /app/site_aggregation /site_aggregation
COPY --from=build /app/link_redirect /link_redirect

CMD ["/intake"]
