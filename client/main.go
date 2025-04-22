package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
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
		//{pb.FileType_IMAGE, "testdata/image-sample.png"},
		{pb.FileType_PDF, "testdata/pdf-sample.pdf"},
		{pb.FileType_PDF, "testdata/blitzer.pdf"},
		//{pb.FileType_VIDEO, "testdata/video-sample.webm"}
	}

	wg := sync.WaitGroup{}

	for _, f := range filePath {
		wg.Add(2)
		go func() {
			defer wg.Done()
			createPreview(f.Path, f.Type, client)
		}()

		go func(f thingy) {
			defer wg.Done()
			createOCR(f.Path, f.Type, client)
		}(f)
	}

	wg.Wait()

}

func createPreview(filePath string, ftype pb.FileType, client pb.ThumbnailServiceClient) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	req := &pb.ThumbnailRequest{
		FileContent: fileContent,
		FileType:    ftype,
		MaxHeight:   150,
	}

	resp, err := client.GenerateThumbnail(context.Background(), req)
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

func createOCR(filePath string, ftype pb.FileType, client pb.ThumbnailServiceClient) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return
	}

	req := &pb.OCRFileRequest{
		FileContent: fileContent,
		FileType:    ftype,
		CleanUp:     true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60000*time.Second)
	defer cancel()

	resp, err := client.OcrFile(ctx, req)
	if err != nil {
		log.Printf("Error calling OcrDocument: %v", err)
		return
	}

	fmt.Printf("[OCR] %s: %s\n %s", filePath, resp.Message, resp.TextContent)

	if len(resp.OcrContent) > 0 {
		err := saveToFile([]byte(resp.OcrContent), filePath, "ocr", ".pdf")
		if err != nil {
			log.Printf("Error saving OCR text to file: %v", err)
		} else {
			fmt.Println("OCR text saved successfully.")
		}
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

func saveToFile(data []byte, originalPath, folder, ext string) error {
	err := os.MkdirAll(folder, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	baseName := filepath.Base(originalPath)
	fileName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	fullPath := filepath.Join(folder, fileName+ext)
	err = os.WriteFile(fullPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	return nil
}
