# Stage 1: Build UI
FROM node:24-alpine3.24 AS ui-build

WORKDIR /app/ui

COPY ui/package.json ui/package-lock.json ./
RUN npm ci

COPY ui ./
RUN npm run build


# Stage 2: Build Go Binary
FROM golang:1.27.1-alpine3.24 AS backend-build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -o conslee ./cmd/conslee


# Stage 3: Final Image
FROM alpine:3.24

RUN apk add --no-cache tzdata

WORKDIR /app

COPY --from=backend-build /app/conslee /usr/local/bin/conslee
COPY --from=ui-build /app/ui/dist /app/ui/dist

VOLUME ["/var/run/docker.sock"]

EXPOSE 8800

ENTRYPOINT ["/usr/local/bin/conslee"]
CMD ["-config", "/app/config/config.yml"]
