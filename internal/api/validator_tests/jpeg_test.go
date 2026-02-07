package validator_tests

import (
	"mime/multipart"
	"net/textproto"
	"testing"

	api "github.com/goatkit/goatflow/internal/api"
)

func TestValidateFile_AllowsJPEGAliases(t *testing.T) {
	hdr := &multipart.FileHeader{
		Filename: "photo.jpg",
		Header:   textproto.MIMEHeader{},
		Size:     1024,
	}
	hdr.Header.Set("Content-Type", "image/pjpeg; charset=binary")

	if err := api.ValidateUploadedFile(hdr); err != nil {
		t.Fatalf("validateFile rejected JPEG alias: %v", err)
	}
}

func TestValidateFile_AllowsPNGAlias(t *testing.T) {
	hdr := &multipart.FileHeader{
		Filename: "diagram.png",
		Header:   textproto.MIMEHeader{},
		Size:     2048,
	}
	hdr.Header.Set("Content-Type", "image/x-png")

	if err := api.ValidateUploadedFile(hdr); err != nil {
		t.Fatalf("validateFile rejected PNG alias: %v", err)
	}
}

func TestValidateFile_BlocksExe(t *testing.T) {
	hdr := &multipart.FileHeader{
		Filename: "malware.exe",
		Header:   textproto.MIMEHeader{},
		Size:     4096,
	}
	hdr.Header.Set("Content-Type", "application/x-msdownload")

	if err := api.ValidateUploadedFile(hdr); err == nil {
		t.Fatalf("validateFile should block .exe but returned nil")
	}
}
