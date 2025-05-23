package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode"
)

func isScannedPDF(path string) (bool, error) {
	content, _, err := extractTextFromPDF(path)
	if err != nil {
		return false, err
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

	tmpOut, err := os.CreateTemp("", "pdftotext-*.txt")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	cmd := exec.Command("pdftotext", path, tmpOut.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("pdftotext failed: %v\nOutput: %s", err, output)
	}

	data, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		return "", nil, fmt.Errorf("failed to read pdftotext output: %w", err)
	}

	rawData, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read raw PDF file: %w", err)
	}

	return string(data), rawData, nil
}

func isUselessLine(line string) bool {
	if len(line) == 0 {
		return true
	}

	firstChar := rune(line[0])
	if !unicode.IsLetter(firstChar) && !unicode.IsNumber(firstChar) {
		allSame := true
		for _, c := range line {
			if c != firstChar {
				allSame = false
				break
			}
		}
		if allSame && len(line) > 3 { // Minimum 4 repeating chars to consider useless
			return true
		}
	}

	return false
}

func cleanOCRText(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))

	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if isUselessLine(line) {
			builder.WriteRune(' ')
			continue
		}

		prevSpace := false
		for _, r := range line {
			if unicode.IsSpace(r) {
				if !prevSpace {
					builder.WriteRune(' ')
					prevSpace = true
				}
			} else {
				builder.WriteRune(r)
				prevSpace = false
			}
		}
	}

	cleaned := strings.TrimSpace(builder.String())
	return cleaned
}
