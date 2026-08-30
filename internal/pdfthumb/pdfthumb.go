package pdfthumb

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// PDF page-1 thumbnails. The platform's gui assets pipeline has no PDF
// rasterizer (libvips here is built without poppler/pdfium), so document
// previews render the first page through poppler's pdftoppm (installed in the
// runtime image). The thumbnail URL contract is identical to images — every
// surface (agent ticket grid, customer list, plugins) requests
// /…/attachments/:id/thumbnail and gets an image/png.
const (
	pdfThumbScaleTo = 400 // max pixel width of the generated page preview
)

var errPDFToolUnavailable = fmt.Errorf("pdftoppm not available (poppler-utils missing)")

// RenderPage1 rasterizes the first page of a PDF to a PNG
// thumbnail (~pdfThumbScaleTo px wide, aspect-preserved) via pdftoppm.
// Returns errPDFToolUnavailable when pdftoppm cannot be found, so callers can
// fall back to their previous non-thumbnail behaviour.
func RenderPage1(content []byte) ([]byte, error) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return nil, errPDFToolUnavailable
	}

	tmp, err := os.CreateTemp("", "gfs-pdf-thumb-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("create temp pdf: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write temp pdf: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close temp pdf: %w", err)
	}

	outRoot := strings.TrimSuffix(tmpPath, ".pdf") // pdftoppm appends .png
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pdftoppm",
		"-png", "-f", "1", "-l", "1", "-singlefile",
		"-scale-to", fmt.Sprintf("%d", pdfThumbScaleTo),
		tmpPath, outRoot)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pdftoppm: %w: %s", err, firstLine(out))
	}

	thumb, err := os.ReadFile(outRoot + ".png")
	if err != nil {
		return nil, fmt.Errorf("read pdf thumbnail: %w", err)
	}
	_ = os.Remove(outRoot + ".png") // best effort
	return thumb, nil
}

func firstLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
