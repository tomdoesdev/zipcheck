package main

import (
	"archive/zip"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileMetadata struct {
	Name             string
	CRC32            uint32
	CompressedSize   uint64
	UncompressedSize uint64
	ModTime          string
}

type CheckResult struct {
	LocalPath string
	ZipPath   string
	Match     bool
	Found     bool
	LocalCRC  uint32
	ZipCRC    uint32
	LocalSize int64
	ZipSize   uint64
}

func calculateFileCRC32(filepath string) (uint32, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return 0, err
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error closing file %s: %v\n", filepath, closeErr)
		}
	}()

	hash := crc32.NewIEEE()
	if _, err := io.Copy(hash, file); err != nil {
		return 0, err
	}

	return hash.Sum32(), nil
}

func getZipMetadata(zipPath string) (map[string]FileMetadata, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error closing reader %v\n", closeErr)
		}
	}()

	metadata := make(map[string]FileMetadata)
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			metadata[file.Name] = FileMetadata{
				Name:             file.Name,
				CRC32:            file.CRC32,
				CompressedSize:   file.CompressedSize64,
				UncompressedSize: file.UncompressedSize64,
				ModTime:          file.Modified.Format("2006-01-02 15:04:05"),
			}
		}
	}

	return metadata, nil
}

func checkFiles(zipPath string, filesToCheck []string) {
	fmt.Printf("\nChecking ZIP file: %s\n", zipPath)
	fmt.Println(strings.Repeat("=", 60))

	// Get ZIP metadata
	zipMetadata, err := getZipMetadata(zipPath)
	if err != nil {
		fmt.Printf("Error reading ZIP file: %v\n", err)
		os.Exit(1)
	}

	if len(zipMetadata) == 0 {
		fmt.Println("No files found in ZIP or unable to read metadata.")
		return
	}

	fmt.Printf("Found %d files in ZIP archive\n\n", len(zipMetadata))

	// Process files and gather results
	results := processFiles(filesToCheck, zipMetadata)

	// Print summary
	printSummary(results)

	// Perform additional analysis
	performAdditionalAnalysis(filesToCheck, zipMetadata)
}

type CheckResults struct {
	Matches    []CheckResult
	Mismatches []CheckResult
	NotFound   []string
}

func processFiles(filesToCheck []string, zipMetadata map[string]FileMetadata) CheckResults {
	var results CheckResults

	for _, filePath := range filesToCheck {
		result := processFile(filePath, zipMetadata)
		if !result.Found {
			results.NotFound = append(results.NotFound, filePath)
		} else if result.Match {
			results.Matches = append(results.Matches, result)
		} else {
			results.Mismatches = append(results.Mismatches, result)
		}
	}

	return results
}

func processFile(filePath string, zipMetadata map[string]FileMetadata) CheckResult {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("❌ Local file not found: %s\n", filePath)
		return CheckResult{LocalPath: filePath}
	}

	filename := filepath.Base(filePath)
	localCRC, err := calculateFileCRC32(filePath)
	if err != nil {
		fmt.Printf("❌ Error calculating CRC for %s: %v\n", filePath, err)
		return CheckResult{LocalPath: filePath}
	}
	localSize := fileInfo.Size()

	fmt.Printf("Checking: %s\n", filename)
	fmt.Printf("  Local CRC-32: 0x%08x\n", localCRC)
	fmt.Printf("  Local size: %s bytes\n", formatNumber(localSize))

	return findFileInZip(filePath, filename, localCRC, localSize, zipMetadata)
}

func findFileInZip(filePath, filename string, localCRC uint32, localSize int64, zipMetadata map[string]FileMetadata) CheckResult {
	for zipName, zipInfo := range zipMetadata {
		zipBasename := filepath.Base(zipName)

		if filename == zipBasename {
			return compareFileWithZip(filePath, zipName, localCRC, localSize, zipInfo)
		}
	}

	fmt.Printf("  ⚠️  No file with name '%s' found in ZIP\n\n", filename)
	return CheckResult{LocalPath: filePath}
}

func compareFileWithZip(filePath, zipName string, localCRC uint32, localSize int64, zipInfo FileMetadata) CheckResult {
	fmt.Printf("  ZIP path: %s\n", zipName)
	fmt.Printf("  ZIP CRC-32: 0x%08x\n", zipInfo.CRC32)
	fmt.Printf("  ZIP size: %s bytes\n", formatNumber(int64(zipInfo.UncompressedSize)))

	result := CheckResult{
		LocalPath: filePath,
		ZipPath:   zipName,
		Found:     true,
		LocalCRC:  localCRC,
		ZipCRC:    zipInfo.CRC32,
		LocalSize: localSize,
		ZipSize:   zipInfo.UncompressedSize,
	}

	if localCRC == zipInfo.CRC32 && uint64(localSize) == zipInfo.UncompressedSize {
		fmt.Printf("  ✅ MATCH - File is identical!\n\n")
		result.Match = true
	} else {
		fmt.Printf("  ❌ MISMATCH - CRC or size differs\n\n")
		result.Match = false
	}

	return result
}

func printSummary(results CheckResults) {
	fmt.Println("\nSUMMARY")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("✅ Matches: %d\n", len(results.Matches))
	for _, match := range results.Matches {
		fmt.Printf("   %s → %s\n", filepath.Base(match.LocalPath), match.ZipPath)
	}

	fmt.Printf("\n❌ Mismatches: %d\n", len(results.Mismatches))
	for _, mismatch := range results.Mismatches {
		fmt.Printf("   %s ≠ %s\n", filepath.Base(mismatch.LocalPath), mismatch.ZipPath)
	}

	fmt.Printf("\n⚠️  Not found in ZIP: %d\n", len(results.NotFound))
	for _, path := range results.NotFound {
		fmt.Printf("   %s\n", filepath.Base(path))
	}
}

func performAdditionalAnalysis(filesToCheck []string, zipMetadata map[string]FileMetadata) {
	fmt.Println("\n\nADDITIONAL ANALYSIS")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Checking for CRC matches with different filenames...")

	for _, filePath := range filesToCheck {
		checkCRCMatches(filePath, zipMetadata)
	}
}

func checkCRCMatches(filePath string, zipMetadata map[string]FileMetadata) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return
	}

	filename := filepath.Base(filePath)
	localCRC, err := calculateFileCRC32(filePath)
	if err != nil {
		return
	}
	localSize := fileInfo.Size()

	var crcMatches []string
	for zipName, zipInfo := range zipMetadata {
		if zipInfo.CRC32 == localCRC &&
			zipInfo.UncompressedSize == uint64(localSize) &&
			filepath.Base(zipName) != filename {
			crcMatches = append(crcMatches, zipName)
		}
	}

	if len(crcMatches) > 0 {
		fmt.Printf("\n'%s' has same CRC/size as:\n", filename)
		for _, match := range crcMatches {
			fmt.Printf("  → %s\n", match)
		}
	}
}

func formatNumber(n int64) string {
	str := fmt.Sprintf("%d", n)
	var result []rune
	for i, r := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, r)
	}
	return string(result)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("ZIP CRC Checker - Verify if files match those in a password-protected ZIP")
		fmt.Println("Usage: zip_crc_check <zipfile> <file1> [file2] [file3] ...")
		os.Exit(1)
	}

	zipPath := os.Args[1]
	filesToCheck := os.Args[2:]

	if _, err := os.Stat(zipPath); err != nil {
		fmt.Printf("Error: ZIP file '%s' not found\n", zipPath)
		os.Exit(1)
	}

	checkFiles(zipPath, filesToCheck)
}
