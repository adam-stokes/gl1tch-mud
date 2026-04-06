FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /app/web/dist web/dist
RUN CGO_ENABLED=0 go build -o gl1tch-mud .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /app/gl1tch-mud /usr/local/bin/
ENTRYPOINT ["gl1tch-mud"]
CMD ["--serve", "--port", "8080"]
