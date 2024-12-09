package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/remeh/sizedwaitgroup" // Install with `go get -u github.com/remeh/sizedwaitgroup`
)

func main() {
	// Define the -path flag to specify the root directory
	rootDir := flag.String("path", ".", "The root directory to process zip files")
	flag.Parse()

	// Define the placeholder PNG file in the root of the working directory
	placeholderPNGPath := "./placeholder.png"

	// Check if the placeholder PNG file exists
	if _, err := os.Stat(placeholderPNGPath); os.IsNotExist(err) {
		fmt.Printf("Placeholder PNG file not found at %s\n", placeholderPNGPath)
		return
	}

	// Create a SizedWaitGroup to limit the number of workers
	numWorkers := runtime.NumCPU()
	swg := sizedwaitgroup.New(numWorkers)

	// Walk through all directories and subdirectories
	err := filepath.Walk(*rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".zip" {
			// Use a goroutine to process each .zip file
			swg.Add()
			go func(zipPath string) {
				defer swg.Done()
				if err := processZipFile(zipPath, placeholderPNGPath); err != nil {
					fmt.Printf("Error processing zip file %s: %v\n", zipPath, err)
					os.Remove(zipPath)
				}
			}(path)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the directory: %v\n", err)
	}

	// Wait for all workers to complete
	swg.Wait()
	fmt.Println("Processing complete.")
}

func processZipFile(zipPath string, placeholderPNGPath string) error {
	// Open the existing zip file
	zipFile, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("could not open zip file: %w", err)
	}
	defer zipFile.Close()

	// Read the zip file content
	zipStat, err := zipFile.Stat()
	if err != nil {
		return fmt.Errorf("could not get zip file info: %w", err)
	}

	zipReader, err := zip.NewReader(zipFile, zipStat.Size())
	if err != nil {
		return fmt.Errorf("could not read zip file: %w", err)
	}

	// Prepare a new zip file buffer
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Read the placeholder PNG
	placeholderPNG, err := os.ReadFile(placeholderPNGPath)
	if err != nil {
		return fmt.Errorf("could not read placeholder PNG: %w", err)
	}

	// Iterate through files in the zip
	for _, file := range zipReader.File {
		if shouldSkipFile(file.Name) {
			fmt.Printf("Excluding file %s\n", file.Name)
			continue
		}

		if filepath.Ext(file.Name) == ".png" {
			// Replace the PNG file with the placeholder
			if err := addPlaceholderPNGToZip(file, zipWriter, placeholderPNG); err != nil {
				return fmt.Errorf("could not replace PNG file: %w", err)
			}
			//fmt.Printf("Replaced PNG file %s with placeholder\n", file.Name)
			continue
		}

		// Copy other files to the new zip
		if err := copyFileToZip(file, zipWriter); err != nil {
			return fmt.Errorf("could not copy file to new zip: %w", err)
		}
	}

	// Close the writer
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("could not close new zip writer: %w", err)
	}

	// Overwrite the original zip file
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("could not overwrite zip file: %w", err)
	}

	return nil
}

func shouldSkipFile(fileName string) bool {
	ext := filepath.Ext(fileName)
	base := filepath.Base(fileName)

	if strings.Contains(fileName, "img-source") {
		return true
	}

	// Exclude .lua, LICENSE, and README.md files
	return ext == ".lua" ||
		ext == ".psd" ||
		ext == ".xcf" ||
		ext == ".blend" ||
		ext == ".jpg" ||

		base == "LICENSE" ||
		base == "README.md" ||
		base == "script.dat" ||
		base == "banner.png" ||
		base == "preview.png" ||
		base == "preview.jpg"
}

func addPlaceholderPNGToZip(file *zip.File, zipWriter *zip.Writer, placeholderPNG []byte) error {
	// Create a new file header with maximum compression
	header := &zip.FileHeader{
		Name:     file.Name,
		Method:   zip.Deflate,
		Modified: file.Modified,
	}

	// Create the new file in the zip
	dstFile, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("could not create file in new zip: %w", err)
	}

	// Write the placeholder PNG content
	_, err = dstFile.Write(placeholderPNG)
	if err != nil {
		return fmt.Errorf("could not write placeholder PNG: %w", err)
	}

	return nil
}

func copyFileToZip(file *zip.File, zipWriter *zip.Writer) error {
	// Open the file inside the zip
	srcFile, err := file.Open()
	if err != nil {
		return fmt.Errorf("could not open file inside zip: %w", err)
	}
	defer srcFile.Close()

	// Create a new file header with maximum compression
	header := &zip.FileHeader{
		Name:     file.Name,
		Method:   zip.Deflate,
		Modified: file.Modified,
	}

	// Create the new file in the zip
	dstFile, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("could not create file in new zip: %w", err)
	}

	// Copy the file content
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("could not copy file content: %w", err)
	}

	return nil
}
