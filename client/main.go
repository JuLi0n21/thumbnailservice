package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	pb "thumbnailclient/proto"

	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	type thingy struct {
		Type pb.FileType
		Path string
	}
	client := pb.NewThumbnailServiceClient(conn)
	filePath := []thingy{
		{pb.FileType_IMAGE, "testdata/image-sample.png"},
		{pb.FileType_PDF, "testdata/pdf-sample.pdf"},
		{pb.FileType_VIDEO, "testdata/video-sample.webm"}}

	for _, f := range filePath {
		newFunction(f.Path, f.Type, client)
	}

}

func newFunction(filePath string, ftype pb.FileType, client pb.ThumbnailServiceClient) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	req := &pb.ThumbnailRequest{
		FileContent: fileContent,
		FileType:    ftype,
		MaxHeight:   150,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	resp, err := client.GenerateThumbnail(ctx, req)
	if err != nil {
		log.Fatalf("Error calling GenerateThumbnail: %v", err)
	}

	fmt.Printf("Response: %s\n", resp.Message)
	if len(resp.ThumbnailContent) > 0 {
		err := saveThumbnailToFile(resp.ThumbnailContent, filePath)
		if err != nil {
			log.Fatalf("Error saving thumbnail to file: %v", err)
		}
		fmt.Println("Thumbnail saved successfully.")
	} else {
		log.Println("No thumbnail content received.")
	}
}

// Function to save the thumbnail content to a file in the 'thumbnail/' directory
func saveThumbnailToFile(thumbnailContent []byte, filePath string) error {
	// Ensure the "thumbnail" directory exists
	err := os.MkdirAll("thumbnail", os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Create the file where the thumbnail will be saved
	baseName := filepath.Base(filePath)
	fileName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	err = os.WriteFile(path.Join("thumbnail", fileName)+".jpg", thumbnailContent, 0644)
	if err != nil {
		return fmt.Errorf("failed to save thumbnail to file: %v", err)
	}

	return nil
}
