package media

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"mime"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp"
)

const (
	DefaultMaxDimension = 3840
	defaultJPEGQuality  = 3
	defaultPNGLevel     = 4
	defaultWebPQuality  = 85
)

type Upload struct {
	Reader      io.Reader
	Size        int64
	FileName    string
	ContentType string
}

type Result struct {
	Bytes       []byte
	ContentType string
	Resized     bool
}

type Processor interface {
	Process(ctx context.Context, upload Upload, maxDimension int) (*Result, error)
}

type FFMPEGProcessor struct {
	path         string
	maxDimension int
	jpegQuality  int
	pngLevel     int
	webpQuality  int
}

func NewFFMPEGProcessor(binaryPath string, maxDimension int) *FFMPEGProcessor {
	path := strings.TrimSpace(binaryPath)
	if path == "" {
		path = "ffmpeg"
	}
	if maxDimension <= 0 {
		maxDimension = DefaultMaxDimension
	}
	return &FFMPEGProcessor{
		path:         path,
		maxDimension: maxDimension,
		jpegQuality:  defaultJPEGQuality,
		pngLevel:     defaultPNGLevel,
		webpQuality:  defaultWebPQuality,
	}
}

func (p *FFMPEGProcessor) Process(ctx context.Context, upload Upload, maxDimension int) (*Result, error) {
	if upload.Reader == nil {
		return nil, fmt.Errorf("media: empty reader")
	}
	data, err := io.ReadAll(upload.Reader)
	if err != nil {
		return nil, fmt.Errorf("media: read image: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("media: empty image data")
	}

	contentType := normalizeContentType(upload.ContentType, upload.FileName)

	width, height, err := decodeDimensions(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("media: decode dimensions: %w", err)
	}
	targetMax := maxDimension
	if targetMax <= 0 {
		targetMax = p.maxDimension
	}
	if width <= targetMax && height <= targetMax {
		return &Result{Bytes: data, ContentType: contentType, Resized: false}, nil
	}

	targetW, targetH := scaleToFit(width, height, targetMax)
	processed, err := p.transcode(ctx, data, contentType, targetW, targetH)
	if err != nil {
		return nil, err
	}

	return &Result{
		Bytes:       processed,
		ContentType: contentType,
		Resized:     true,
	}, nil
}

func decodeDimensions(r io.Reader) (int, int, error) {
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return 0, 0, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, fmt.Errorf("invalid dimensions %dx%d", cfg.Width, cfg.Height)
	}
	return cfg.Width, cfg.Height, nil
}

func scaleToFit(width, height, maxDim int) (int, int) {
	if width >= height {
		newW := maxDim
		newH := int(math.Round(float64(height) * float64(maxDim) / float64(width)))
		return ensureMin(newW), ensureMin(newH)
	}
	newH := maxDim
	newW := int(math.Round(float64(width) * float64(maxDim) / float64(height)))
	return ensureMin(newW), ensureMin(newH)
}

func ensureMin(value int) int {
	if value < 2 {
		return 2
	}
	return value
}

func (p *FFMPEGProcessor) transcode(ctx context.Context, data []byte, contentType string, width, height int) ([]byte, error) {
	codec, args, err := p.codecArgs(contentType)
	if err != nil {
		return nil, err
	}

	scaleArg := fmt.Sprintf("scale=%d:%d:flags=lanczos", width, height)
	cmdArgs := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-vf", scaleArg,
		"-frames:v", "1",
		"-f", "image2",
		"-c:v", codec,
	}
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, "pipe:1")

	cmd := exec.CommandContext(ctx, p.path, cmdArgs...)
	cmd.Stdin = bytes.NewReader(data)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("ffmpeg: %v: %s", err, errMsg)
		}
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}

	result := stdout.Bytes()
	if len(result) == 0 {
		return nil, fmt.Errorf("ffmpeg: produced empty output")
	}
	return result, nil
}

func (p *FFMPEGProcessor) codecArgs(contentType string) (string, []string, error) {
	switch contentType {
	case "image/jpeg", "image/jpg":
		return "mjpeg", []string{"-q:v", strconv.Itoa(p.jpegQuality)}, nil
	case "image/png":
		return "png", []string{"-compression_level", strconv.Itoa(p.pngLevel)}, nil
	case "image/webp":
		return "libwebp", []string{"-quality", strconv.Itoa(p.webpQuality)}, nil
	default:
		return "", nil, fmt.Errorf("media: unsupported content type %s", contentType)
	}
}

func normalizeContentType(value, fileName string) string {
	ct := strings.ToLower(strings.TrimSpace(value))
	if ct != "" {
		if ct == "image/jpg" {
			return "image/jpeg"
		}
		return ct
	}
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(fileName)))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	}
	if ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			return strings.ToLower(mt)
		}
	}
	return "image/jpeg"
}
