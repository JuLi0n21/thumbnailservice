FROM golang:1.24.2

WORKDIR /app

RUN apt-get update && apt-get install -y \
    imagemagick \
    ffmpeg \
    poppler-utils \
    ocrmypdf \
    qpdf \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY . .

EXPOSE 50051

RUN go mod tidy && go build -o thumbnail-service

CMD ["./thumbnail-service"]