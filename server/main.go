package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"image/gif"
	_ "image/gif"
	"image/jpeg"
	"image/png"

	pb "github.com/JuLi0n21/thumbnail_service/proto"
	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
	"github.com/nfnt/resize"
	"google.golang.org/grpc"
)

func generateVideoThumbnail(inputPath, outputPath string, maxWidth, maxHeight int) error {
	cmd := exec.Command("ffmpeg", "-y", "-i", inputPath, "-vf", "thumbnail", "-frames:v", "1", outputPath)
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

func generatePdfThumbnail(inputPath, outputPath string, maxWidth, maxHeight int) error {

	filename := strings.TrimSuffix(outputPath, ".jpg")

	cmd := exec.Command("pdftoppm",
		inputPath, filename, "-jpeg",
		"-singlefile",
		"-f", "1",
		"-l", "1",
		"-scale-to", strconv.Itoa(maxHeight))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate PDF thumbnail using Poppler-utils: %v", err)
	}

	if maxWidth > 0 || maxHeight > 0 {
		return resizeImage(outputPath, outputPath, maxWidth, maxHeight)
	}

	return nil
}

func resizeImage(inputPath, outputPath string, maxWidth, maxHeight int) error {

	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open image file: %v", err)
	}
	defer file.Close()

	img, imgType, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %v", err)
	}

	var newWidth, newHeight int
	if maxWidth > 0 && maxHeight > 0 {

		newWidth = maxWidth
		newHeight = maxHeight
	} else if maxWidth > 0 {

		newWidth = maxWidth
		newHeight = int(float64(img.Bounds().Dy()) * float64(maxWidth) / float64(img.Bounds().Dx()))
	} else if maxHeight > 0 {

		newHeight = maxHeight
		newWidth = int(float64(img.Bounds().Dx()) * float64(maxHeight) / float64(img.Bounds().Dy()))
	} else {

		newWidth = img.Bounds().Dx()
		newHeight = img.Bounds().Dy()
	}

	resizedImg := resize.Resize(uint(newWidth), uint(newHeight), img, resize.Lanczos3)

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
	case "gif":
		err = gif.Encode(outFile, resizedImg, &gif.Options{})
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
	start := time.Now()
	fmt.Println(start.Format("2006-01-02 15:04:05.000"), "Thumbnail request ", req.FileType, "H: ", req.MaxHeight, "W: ", req.MaxWidth)

	tempFile, err := os.CreateTemp("", "upload-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	err = os.WriteFile(tempFile.Name(), req.FileContent, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write content to file: %v", err)
	}

	thumbnailName := fmt.Sprintf("thumbnail-%s.jpg", uuid.New().String())
	outputPath := filepath.Join("thumbnails", thumbnailName)

	if err := os.MkdirAll("thumbnails", 0755); err != nil {
		return nil, fmt.Errorf("failed to create thumbnails directory: %v", err)
	}

	switch req.FileType {
	case pb.FileType_IMAGE:
		err = resizeImage(tempFile.Name(), outputPath, int(req.MaxWidth), int(req.MaxHeight))
	case pb.FileType_VIDEO:
		err = generateVideoThumbnail(tempFile.Name(), outputPath, int(req.MaxWidth), int(req.MaxHeight))
	case pb.FileType_PDF:
		err = generatePdfThumbnail(tempFile.Name(), outputPath, int(req.MaxWidth), int(req.MaxHeight))
	default:
		return nil, fmt.Errorf("unsupported file type: %v", req.FileType)
	}

	if err != nil {
		return nil, err
	}

	defer func() {
		if _, err := os.Stat(outputPath); err == nil {
			os.Remove(outputPath)
		}
		end := time.Since(time.Now())
		fmt.Println(time.Now().Format("2006-01-02 15:04:05.000"), "Finshed in: ", end, req.FileType, "H: ", req.MaxHeight, "W: ", req.MaxWidth)
	}()

	thumbnailContent, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated thumbnail: %v", err)
	}

	return &pb.ThumbnailResponse{
		Message:          "Thumbnail generated successfully",
		ThumbnailContent: thumbnailContent,
	}, nil
}

func (s *server) OcrFile(ctx context.Context, req *pb.OCRFileRequest) (*pb.OCRFileResponse, error) {
	start := time.Now()
	fmt.Println(start.Format("2006-01-02 15:04:05.000"), "OCR request ", req.FileType)

	defer func() {
		end := time.Since(time.Now())
		fmt.Println(time.Now().Format("2006-01-02 15:04:05.000"), "OCR Finshed in: ", end, req.FileType)
	}()
	if req.FileType != pb.FileType_PDF {
		err := errors.New("unsupported Filetype " + req.FileType.String())
		return &pb.OCRFileResponse{
			Message:     "OCR failed, " + err.Error(),
			TextContent: "",
			OcrContent:  []byte{},
		}, err
	}
	file, err := os.CreateTemp("ocr", "temp-file-*")
	if err != nil {
		return &pb.OCRFileResponse{
			Message:     "OCR failed, " + err.Error(),
			TextContent: "",
			OcrContent:  []byte{},
		}, err
	}
	filePath := file.Name()
	defer func(file *os.File, filePath string) {
		file.Close()

		err = os.Remove(file.Name())
		if err != nil {
			fmt.Println(err.Error())
		}
	}(file, filePath)

	_, err = file.Write(req.FileContent)
	if err != nil {
		return &pb.OCRFileResponse{
			Message:     "OCR failed, " + err.Error(),
			TextContent: "",
			OcrContent:  []byte{},
		}, err
	}

	if ok, err := isScannedPDF(filePath); err != nil {
		return &pb.OCRFileResponse{
			Message:     "OCR failed, " + err.Error(),
			TextContent: "",
			OcrContent:  []byte{},
		}, err
	} else if !ok {
		if isEncrypted(filePath) {
			err := decryptPDF(filePath)
			if err != nil {
				return &pb.OCRFileResponse{
					Message:     "OCR failed, " + err.Error(),
					TextContent: "",
					OcrContent:  []byte{},
				}, err
			}
		}

		err = runOCRMyPDF(filePath)
		if err != nil {
			return &pb.OCRFileResponse{
				Message:     "OCR failed, " + err.Error(),
				TextContent: "",
				OcrContent:  []byte{},
			}, err
		}
	}

	var text string
	var b []byte
	text, b, err = extractTextFromPDF(filePath)
	if err != nil {

		if strings.Contains("malformed pdf", err.Error()) {
			repairPDF(filePath)
			text, b, err = extractTextFromPDF(filePath)

		} else {
			return &pb.OCRFileResponse{
				Message:     "OCR failed, " + err.Error(),
				TextContent: "",
				OcrContent:  []byte{},
			}, err
		}
	}

	return &pb.OCRFileResponse{
		Message:     "OCR success",
		TextContent: text,
		OcrContent:  b,
	}, nil
}

func isScannedPDF(path string) (bool, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer f.Close()

	reader, err := r.GetPlainText()
	if err != nil {
		return false, fmt.Errorf("failed to get PDF text: %w", err)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return false, fmt.Errorf("failed to read PDF content: %w", err)
	}

	return len(strings.TrimSpace(string(content))) != 0, nil
}

func runOCRMyPDF(inputPath string) error {
	tempfile, err := os.CreateTemp("", "temp-ocr-*.pdf")
	defer os.Remove(tempfile.Name())
	if err != nil {
		return err
	}
	cmd := exec.Command("ocrmypdf", "--skip-text", inputPath, tempfile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ocrmypdf failed: %v\nOutput: %s", err, output)
	}

	processedData, err := os.ReadFile(tempfile.Name())
	if err != nil {
		return fmt.Errorf("failed to read processed file: %v", err)
	}

	err = os.WriteFile(inputPath, processedData, 0644)
	if err != nil {
		return fmt.Errorf("failed to overwrite input file: %v", err)
	}
	return nil
}

func isEncrypted(pdfPath string) bool {
	cmd := exec.Command("qpdf", "--check", pdfPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.Contains(string(output), "File is not encrypted")
	}
	return false
}

func repairPDF(inputPath string) error {
	tempfile, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	defer os.Remove(tempfile.Name())

	cmd := exec.Command("qpdf", "--repair", inputPath, tempfile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	_, err = tempfile.Write(output)

	return err
}

func decryptPDF(inputPath string) error {
	tempfile, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	defer os.Remove(tempfile.Name())

	cmd := exec.Command("qpdf", "--decrypt", inputPath, tempfile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qpdf failed: %v\nOutput: %s", err, output)
	}

	processedData, err := os.ReadFile(tempfile.Name())
	if err != nil {
		return fmt.Errorf("failed to read processed file: %v", err)
	}

	err = os.WriteFile(inputPath, processedData, 0644)
	if err != nil {
		return fmt.Errorf("failed to overwrite input file: %v", err)
	}
	return nil
}

func extractTextFromPDF(path string) (string, []byte, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", []byte{}, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer f.Close()

	reader, err := r.GetPlainText()
	if err != nil {
		return "", []byte{}, fmt.Errorf("failed to get PDF text: %w", err)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", []byte{}, fmt.Errorf("failed to read PDF content: %w", err)
	}

	rawData, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read raw PDF file: %w", err)
	}

	return string(content), rawData, nil
}

const maxMsgSize = 2147483648 // 2GB

func main() {

	listen, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMsgSize),
		grpc.MaxSendMsgSize(maxMsgSize))

	pb.RegisterThumbnailServiceServer(grpcServer, &server{})

	log.Println("Server started on port 50051")
	if err := grpcServer.Serve(listen); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
