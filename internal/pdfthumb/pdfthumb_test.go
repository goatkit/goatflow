package pdfthumb

import (
	"bytes"
	"os/exec"
	"testing"
)

// minimalPDF is a self-contained, well-formed one-page PDF (classic "hello
// world" sample) used to exercise the rasterizer without fixtures on disk.
const minimalPDF = `%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>endobj
4 0 obj<</Length 70>>stream
BT /F1 24 Tf 72 720 Td (Hello PDF thumbnail) Tj ET
endstream
endobj
5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000052 00000 n 
0000000101 00000 n 
0000000206 00000 n 
0000000323 00000 n 
trailer<</Size 6/Root 1 0 R>>
startxref
386
%%EOF`

func TestRenderPdfPage1Thumbnail(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not installed; skipping PDF thumbnail raster test")
	}
	png, err := RenderPage1([]byte(minimalPDF))
	if err != nil {
		t.Fatalf("RenderPage1: %v", err)
	}
	if !bytes.HasPrefix(png, []byte("\x89PNG")) {
		t.Fatalf("output is not PNG: got %q", png[:min(8, len(png))])
	}
	if len(png) < 100 {
		t.Fatalf("thumbnail suspiciously small: %d bytes", len(png))
	}
}

func TestRenderPdfPage1ThumbnailRejectsGarbage(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not installed; skipping")
	}
	if _, err := RenderPage1([]byte("this is not a pdf")); err == nil {
		t.Fatal("expected error for non-PDF input")
	}
}
