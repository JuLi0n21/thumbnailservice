FROM golang:1.24.2 AS builder

WORKDIR /app

COPY . .

RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o thumbnail-service

FROM alpine:3.21

RUN apk add --no-cache python3 py3-pip

RUN apk add --no-cache ghostscript

RUN apk add --no-cache tesseract-ocr tesseract-ocr-data-eng tesseract-ocr-data-deu

RUN apk add --no-cache x264-libs x265-libs libvpx dav1d aom-libs

RUN apk add --no-cache ffmpeg

RUN apk add --no-cache imagemagick poppler-utils qpdf

RUN apk add --no-cache py3-pillow py3-reportlab py3-pikepdf py3-cryptography

RUN apk add --no-cache ocrmypdf

WORKDIR /app

COPY --from=builder /app/thumbnail-service .

EXPOSE 50051

CMD ["./thumbnail-service"]
