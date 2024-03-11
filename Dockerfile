FROM golang:1.19-alpine AS go-build

RUN mkdir /src
WORKDIR /src
RUN apk add --update --no-cache ca-certificates git

ENV GO111MODULE=on

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine

RUN apk --no-cache --update add ca-certificates
WORKDIR /app
COPY --from=go-build /src/main /app/

ENTRYPOINT ./main