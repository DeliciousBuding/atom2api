# Stage 1: Build frontend
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: Build Go binary
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/frontend/dist frontend/dist
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o atom2api .

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /app/atom2api .
RUN mkdir -p /data
EXPOSE 8080
ENTRYPOINT ["./atom2api"]
