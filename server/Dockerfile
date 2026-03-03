FROM golang:1.26.0 AS builder

WORKDIR /build
COPY go.mod go.sum ./

RUN go mod download
COPY . .

RUN go build -o /app/thumbnail-service

FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update
RUN apt-get install -y --no-install-recommends imagemagick
RUN apt-get install -y --no-install-recommends ffmpeg
RUN apt-get install -y --no-install-recommends poppler-utils
RUN apt-get install -y --no-install-recommends qpdf
RUN apt-get install -y --no-install-recommends ocrmypdf 

RUN apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/thumbnail-service .

EXPOSE 50051
CMD ["./thumbnail-service"]
