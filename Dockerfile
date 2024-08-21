# go build
FROM golang:1.19-alpine3.16 as builder
RUN apk update \
    && apk upgrade \
    && apk add --no-cache \
	ca-certificates \
	&& update-ca-certificates 2>/dev/null || true
WORKDIR /app
COPY ./go.mod ./
COPY ./go.sum ./
COPY ./main.go ./
RUN go build -o ./escargo .

# runtime build
FROM golang:1.19-alpine3.16
RUN apk update \
    && apk upgrade \
    && apk add --no-cache \
	ca-certificates \
    yq \
	&& update-ca-certificates 2>/dev/null || true
WORKDIR /
COPY --from=builder /app/escargo /escargo
CMD ["/escargo"]