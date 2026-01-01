package api

import (
	"mime/multipart"
	"net/textproto"
	"testing"
)

func TestValidateFile_AllowsWebP(t *testing.T) {
	hdr := &multipart.FileHeader{
		Filename: "image.webp",
		Header:   textproto.MIMEHeader{},
		Size:     1500,
	}
	hdr.Header.Set("Content-Type", "image/webp")
	if err := ValidateUploadedFile(hdr); err != nil {
		t.Fatalf("validateFile rejected WebP: %v", err)
	}
}

func TestValidateFile_AllowsSVGAndNormalizes(t *testing.T) {
	hdr := &multipart.FileHeader{
		Filename: "vector.svg",
		Header:   textproto.MIMEHeader{},
		Size:     1200,
	}
	// Some agents send image/svg
	hdr.Header.Set("Content-Type", "image/svg; charset=utf-8")
	if err := ValidateUploadedFile(hdr); err != nil {
		t.Fatalf("validateFile rejected SVG alias: %v", err)
	}
}
