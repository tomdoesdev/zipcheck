# zipcheck

zipcheck is a command-line tool that validates the existence of known files in a password-protected ZIP archive without requiring the password. It works by comparing the CRC-32 checksums and file sizes of local files with those stored in the ZIP file header, which can be read without decryption.

## Features

- Verify if local files match those in a password-protected ZIP archive
- Compare file checksums (CRC-32) and sizes
- Find files in the ZIP that match your local files, even if they have different names
- Support for multiple file checking in a single run
- Detailed and summary reports of matches, mismatches, and missing files

## Requirements

- Any version of Go compatible with v1.24 (developed with go1.24.3)

## Installation

### Build from Source

Clone the repository and build using Go:

```sh
git clone https://github.com/tomdoesdev/zipcheck.git
cd zipcheck
go build
```

This will create a zipcheck binary in the current directory.

## Useage
```sh
./zipcheck <zipfile> <file1> [file2] [file3] ...
```

Example:
```sh
./zipcheck archive.zip document.pdf image.jpg
```

You can also use shell globbing patterns to check multiple files:
```sh
./zipcheck archive.zip *.pdf *.jpg
```


## How it works

zipcheck leverges the fact that ZIP file headers contain CRC-32 checksums and size information for each file in the archive, even when the actual file content is password protected. By comparing these values with those of local files, zipcheck can determine if a local file exists inside the ZIP archive with high confidence without needing the password.