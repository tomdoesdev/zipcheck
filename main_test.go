package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type TestFileFixture struct {
	Name    string
	Content []byte
	CRC32   uint32 // Will be calculated when creating the file
	Size    int64  // Will be calculated when creating the file
}

// createTestFiles creates temporary files for testing
func createTestFiles(t *testing.T) ([]string, []TestFileFixture) {
	testFiles := []TestFileFixture{
		{Name: "document.txt", Content: []byte("This is a test document")},
		{Name: "empty.txt", Content: []byte("")},
		{Name: "image.dat", Content: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}}, // Fake image data
	}

	tempDir, err := os.MkdirTemp("", "zipcheck-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	var filePaths []string

	for i := range testFiles {
		filePath := filepath.Join(tempDir, testFiles[i].Name)
		err := os.WriteFile(filePath, testFiles[i].Content, 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		filePaths = append(filePaths, filePath)

		// Calculate CRC32 and size
		crc, err := calculateFileCRC32(filePath)
		if err != nil {
			t.Fatalf("Failed to calculate CRC32: %v", err)
		}
		testFiles[i].CRC32 = crc
		testFiles[i].Size = int64(len(testFiles[i].Content))
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return filePaths, testFiles
}

// createTestZip creates a zip archive with the provided files
func createTestZip(t *testing.T, files []TestFileFixture, password bool) string {
	// Note: Go's standard zip package doesn't support encryption so were just creating a standard zip
	zipPath := filepath.Join(t.TempDir(), "test.zip")

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, file := range files {
		w, err := zipWriter.Create(file.Name)
		if err != nil {
			t.Fatalf("Failed to create entry in zip: %v", err)
		}
		_, err = w.Write(file.Content)
		if err != nil {
			t.Fatalf("Failed to write content to zip: %v", err)
		}
	}

	return zipPath
}

// createZipWithRenamedFiles creates a zip with files that have different names but same content
func createZipWithRenamedFiles(t *testing.T, files []TestFileFixture) string {
	renamedFiles := make([]TestFileFixture, len(files))
	copy(renamedFiles, files)

	// Rename files in the archive but keep the same content
	for i := range renamedFiles {
		renamedFiles[i].Name = "renamed_" + renamedFiles[i].Name
	}

	return createTestZip(t, renamedFiles, false)
}

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// TestCalculateFileCRC32 tests the CRC32 calculation function
func TestCalculateFileCRC32(t *testing.T) {
	filePaths, fixtures := createTestFiles(t)

	for i, path := range filePaths {
		crc, err := calculateFileCRC32(path)
		if err != nil {
			t.Fatalf("Failed to calculate CRC32: %v", err)
		}

		if crc != fixtures[i].CRC32 {
			t.Errorf("CRC32 mismatch for %s: got %d, expected %d",
				path, crc, fixtures[i].CRC32)
		}
	}
}

// TestGetZipMetadata tests extracting metadata from ZIP files
func TestGetZipMetadata(t *testing.T) {
	_, fixtures := createTestFiles(t)
	zipPath := createTestZip(t, fixtures, false)

	metadata, err := getZipMetadata(zipPath)
	if err != nil {
		t.Fatalf("Failed to get zip metadata: %v", err)
	}

	if len(metadata) != len(fixtures) {
		t.Errorf("Expected %d files in metadata, got %d", len(fixtures), len(metadata))
	}

	for _, file := range fixtures {
		meta, exists := metadata[file.Name]
		if !exists {
			t.Errorf("File %s not found in zip metadata", file.Name)
			continue
		}

		if meta.CRC32 != file.CRC32 {
			t.Errorf("CRC32 mismatch for %s: got %d, expected %d",
				file.Name, meta.CRC32, file.CRC32)
		}

		if meta.UncompressedSize != uint64(file.Size) {
			t.Errorf("Size mismatch for %s: got %d, expected %d",
				file.Name, meta.UncompressedSize, file.Size)
		}
	}
}

// TestCheckFiles_ExactMatches tests checking files with exact matches
func TestCheckFiles_ExactMatches(t *testing.T) {
	filePaths, fixtures := createTestFiles(t)
	zipPath := createTestZip(t, fixtures, false)

	output := captureOutput(func() {
		checkFiles(zipPath, filePaths)
	})

	// Verify output contains expected results
	if !strings.Contains(output, "✅ Matches: 3") {
		t.Errorf("Expected 3 matches, output was: %s", output)
	}

	for _, file := range fixtures {
		if !strings.Contains(output, "✅ MATCH - File is identical!") {
			t.Errorf("Expected match for %s, not found in output: %s", file.Name, output)
		}
	}
}

// TestCheckFiles_Mismatches tests handling files with same names but different content
func TestCheckFiles_Mismatches(t *testing.T) {
	filePaths, fixtures := createTestFiles(t)

	// Create modified copies of the files
	modifiedFixtures := make([]TestFileFixture, len(fixtures))
	copy(modifiedFixtures, fixtures)

	// Modify content to create mismatches but keep names the same
	for i := range modifiedFixtures {
		modifiedFixtures[i].Content = append(modifiedFixtures[i].Content, []byte("modified")...)
	}

	zipPath := createTestZip(t, modifiedFixtures, false)

	output := captureOutput(func() {
		checkFiles(zipPath, filePaths)
	})

	if !strings.Contains(output, "❌ Mismatches: 3") {
		t.Errorf("Expected 3 mismatches, output was: %s", output)
	}

	for _, file := range fixtures {
		if !strings.Contains(output, "❌ MISMATCH - CRC or size differs") {
			t.Errorf("Expected mismatch for %s, not found in output: %s", file.Name, output)
		}
	}
}

// TestCheckFiles_MissingFiles tests handling files not found in the archive
func TestCheckFiles_MissingFiles(t *testing.T) {
	filePaths, fixtures := createTestFiles(t)

	// Create zip with only a subset of files
	zipPath := createTestZip(t, fixtures[:1], false)

	output := captureOutput(func() {
		checkFiles(zipPath, filePaths)
	})

	if !strings.Contains(output, "⚠️  Not found in ZIP: 2") {
		t.Errorf("Expected 2 missing files, output was: %s", output)
	}

	for _, file := range fixtures[1:] {
		if !strings.Contains(output, "⚠️  No file with name '"+file.Name+"' found in ZIP") {
			t.Errorf("Expected file %s to be missing, not indicated in output: %s", file.Name, output)
		}
	}
}

// TestCheckFiles_DifferentNames tests finding files with different names but same content
func TestCheckFiles_DifferentNames(t *testing.T) {
	filePaths, fixtures := createTestFiles(t)
	zipPath := createZipWithRenamedFiles(t, fixtures)

	output := captureOutput(func() {
		checkFiles(zipPath, filePaths)
	})

	if !strings.Contains(output, "has same CRC/size as:") {
		t.Errorf("Expected to find files with different names but same content, output was: %s", output)
	}

	for _, file := range fixtures {
		if !strings.Contains(output, "renamed_"+file.Name) {
			t.Errorf("Expected to find renamed version of %s, not found in output: %s", file.Name, output)
		}
	}
}

// TestFormatNumber tests the number formatting function
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{123, "123"},
		{1234, "1,234"},
		{12345678, "12,345,678"},
		{0, "0"},
	}

	for _, test := range tests {
		result := formatNumber(test.input)
		if result != test.expected {
			t.Errorf("formatNumber(%d) = %s, expected %s", test.input, result, test.expected)
		}
	}
}
