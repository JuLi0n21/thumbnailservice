package main

import (
	"context"
	"fmt"
	"image"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	_ "image/png"

	pb "github.com/JuLi0n21/thumbnail_service/proto"
	"github.com/nfnt/resize"
	"google.golang.org/grpc"
)

// Helper function to generate video thumbnails using ffmpeg
func generateVideoThumbnail(inputPath, outputPath string, maxWidth, maxHeight int) error {
	cmd := exec.Command("ffmpeg", "-i", inputPath, "-vf", "thumbnail", "-frames:v", "1", outputPath)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("failed to generate video thumbnail using FFmpeg: %v. FFmpeg stderr: %s", err, stderr.String())
	}

	if maxWidth > 0 || maxHeight > 0 {
		return resizeImage(outputPath, outputPath, maxWidth, maxHeight)
	}
	return nil
}

// Helper function to generate PDF thumbnails using poppler-utils (pdftoppm)
func generatePdfThumbnail(inputPath, outputPath string, maxWidth, maxHeight int) error {
	// Command for Poppler-utils to generate a thumbnail from the first page of a PDF file
	cmd := exec.Command("pdftoppm", inputPath, outputPath, "-jpeg", "-f", "1", "-l", "1", "-scale-to", "200")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate PDF thumbnail using Poppler-utils: %v", err)
	}

	outputFileWithPage := fmt.Sprintf("%s-01.jpg", outputPath) // pdftoppm output file with page number suffix

	// Rename the file
	err = os.Rename(outputFileWithPage, outputPath)
	if err != nil {
		return fmt.Errorf("failed to rename file: %v", err)
	}

	// Resize the generated image if maxWidth or maxHeight is provided
	if maxWidth > 0 || maxHeight > 0 {
		return resizeImage(outputPath, outputPath, maxWidth, maxHeight)
	}

	return nil
}

func resizeImage(inputPath, outputPath string, maxWidth, maxHeight int) error {
	// Open the input image file
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open image file: %v", err)
	}
	defer file.Close()

	// Decode the image
	img, imgType, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %v", err)
	}

	// Calculate the new dimensions, preserving the aspect ratio
	var newWidth, newHeight int
	if maxWidth > 0 && maxHeight > 0 {
		// Resize with both width and height limit
		newWidth = maxWidth
		newHeight = maxHeight
	} else if maxWidth > 0 {
		// Resize based on width
		newWidth = maxWidth
		newHeight = int(float64(img.Bounds().Dy()) * float64(maxWidth) / float64(img.Bounds().Dx()))
	} else if maxHeight > 0 {
		// Resize based on height
		newHeight = maxHeight
		newWidth = int(float64(img.Bounds().Dx()) * float64(maxHeight) / float64(img.Bounds().Dy()))
	} else {
		// No resizing needed
		newWidth = img.Bounds().Dx()
		newHeight = img.Bounds().Dy()
	}

	// Resize the image using the calculated dimensions
	resizedImg := resize.Resize(uint(newWidth), uint(newHeight), img, resize.Lanczos3)

	// Create the output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()
	switch imgType {
	case "jpeg":
		err = jpeg.Encode(outFile, resizedImg, nil)
		if err != nil {
			return fmt.Errorf("failed to save resized image as jpeg: %v", err)
		}
	case "png":
		err = png.Encode(outFile, resizedImg)
		if err != nil {
			return fmt.Errorf("failed to save resized image as png: %v", err)
		}
	default:
		return fmt.Errorf("unsupported image type: %v", imgType)
	}

	return nil
}

type server struct {
	pb.UnimplementedThumbnailServiceServer
}

func (s *server) GenerateThumbnail(ctx context.Context, req *pb.ThumbnailRequest) (*pb.ThumbnailResponse, error) {
	// Create a temporary file to store the uploaded content
	tempFile, err := os.CreateTemp("", "upload-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name()) // Cleanup temporary file

	// Write the content from the request to the temporary file
	err = os.WriteFile(tempFile.Name(), req.FileContent, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write content to file: %v", err)
	}

	// Generate thumbnail and save it to disk based on file type
	var outputPath string

	// Check file type using enum
	switch req.FileType {
	case pb.FileType_IMAGE:
		// Image file, use ImageMagick or other Go logic to create a thumbnail
		outputPath = "thumbnails/image-thumbnail.jpg"
		err = resizeImage(tempFile.Name(), outputPath, int(req.MaxWidth), int(req.MaxHeight))
		if err != nil {
			return nil, err
		}

	case pb.FileType_VIDEO:
		// Video file, use FFmpeg to create a thumbnail
		outputPath = "thumbnails/video-thumbnail.jpg" // Video thumbnails are typically saved as JPG
		err = generateVideoThumbnail(tempFile.Name(), outputPath, int(req.MaxWidth), int(req.MaxHeight))
		if err != nil {
			return nil, err
		}

	case pb.FileType_PDF:
		// PDF file, use Poppler-utils to create a thumbnail
		outputPath = "thumbnails/pdf-thumbnail.jpg" // PDF thumbnails are typically saved as JPG
		err = generatePdfThumbnail(tempFile.Name(), outputPath, int(req.MaxWidth), int(req.MaxHeight))
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported file type: %v", req.FileType)
	}

	// Read the generated thumbnail back into memory to send it as bytes
	thumbnailContent, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated thumbnail: %v", err)
	}

	// Return the response with the thumbnail bytes and output path
	return &pb.ThumbnailResponse{
		Message:          "Thumbnail generated successfully",
		ThumbnailContent: thumbnailContent, // Send the thumbnail as bytes
	}, nil
}

func main() {
	// Set up a listener on port 50051
	listen, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create a gRPC server
	grpcServer := grpc.NewServer()

	// Register the server
	pb.RegisterThumbnailServiceServer(grpcServer, &server{})

	// Start serving requests
	log.Println("Server started on port 50051")
	if err := grpcServer.Serve(listen); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
