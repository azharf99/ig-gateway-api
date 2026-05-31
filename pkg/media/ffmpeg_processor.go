package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type EditMetadata struct {
	Text         string `json:"text"`
	TextStyle    string `json:"text_style"`    // "classic", "neon", "typewriter"
	TextPosition string `json:"text_position"` // "upper-third", "center", "lower-third"
	LogoPosition string `json:"logo_position"` // "top-center", "bottom-center"
	LogoScale    int    `json:"logo_scale"`    // 10 to 100 percentage
	MuteAudio    bool   `json:"mute_audio"`
	HasAudio     bool   `json:"has_audio"`
	HasLogo      bool   `json:"has_logo"`
	HasSubtitles bool   `json:"has_subtitles"`
}

func getFontPath() string {
	if runtime.GOOS == "windows" {
		// Try standard Windows paths
		paths := []string{
			"C:/Windows/Fonts/arial.ttf",
			"C:/Windows/Fonts/segoeui.ttf",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		return "Arial" // fallback
	}

	// Try Alpine/Ubuntu paths
	linuxPaths := []string{
		"/usr/share/fonts/ttf/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/freefont/FreeSans.ttf",
	}
	for _, p := range linuxPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "" // let ffmpeg try its fallback
}

func escapeFFmpegText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "'\\\\''")
	s = strings.ReplaceAll(s, ":", "\\:")
	s = strings.ReplaceAll(s, "%", "\\%")
	return s
}

func escapeFFmpegFilterPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.ReplaceAll(p, ":", "\\:")
	p = strings.ReplaceAll(p, "'", "'\\\\''")
	return p
}

func wordWrap(text string, maxLineLength int) string {
	lines := strings.Split(text, "\n")
	var wrappedLines []string
	for _, line := range lines {
		words := strings.Fields(line)
		if len(words) == 0 {
			wrappedLines = append(wrappedLines, "")
			continue
		}
		var wrapped string
		currentLineLength := 0
		for _, word := range words {
			if currentLineLength > 0 {
				if currentLineLength+1+len(word) > maxLineLength {
					wrapped += "\n"
					currentLineLength = 0
				} else {
					wrapped += " "
					currentLineLength++
				}
			}
			wrapped += word
			currentLineLength += len(word)
		}
		wrappedLines = append(wrappedLines, wrapped)
	}
	return strings.Join(wrappedLines, "\n")
}

func GetVideoDuration(videoPath string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration error: %w", err)
	}
	durationStr := strings.TrimSpace(out.String())
	return strconv.ParseFloat(durationStr, 64)
}

func HasAudioStream(videoPath string) (bool, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "a", "-show_entries", "stream=codec_type", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return false, fmt.Errorf("ffprobe audio stream check error: %w", err)
	}
	return strings.TrimSpace(out.String()) != "", nil
}

func GetVideoWidth(videoPath string) (int, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("ffprobe width error: %w", err)
	}
	widthStr := strings.TrimSpace(out.String())
	width, err := strconv.Atoi(widthStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse width: %w", err)
	}
	return width, nil
}

func ProcessVideo(ctx context.Context, videoPath, audioPath, logoPath, subtitlePath string, meta EditMetadata, outputPath string) error {
	duration, err := GetVideoDuration(videoPath)
	if err != nil {
		return fmt.Errorf("failed to get video duration: %w", err)
	}

	hasOrigAudio, err := HasAudioStream(videoPath)
	if err != nil {
		return fmt.Errorf("failed to check original audio stream: %w", err)
	}

	mainWidth := 1080 // fallback
	if w, err := GetVideoWidth(videoPath); err == nil && w > 0 {
		mainWidth = w
	}

	args := []string{"-y"} // overwrite output file

	// Add inputs
	args = append(args, "-i", videoPath)

	logoIdx := -1
	audioIdx := -1
	currentInputIdx := 1

	if meta.HasLogo && logoPath != "" {
		args = append(args, "-i", logoPath)
		logoIdx = currentInputIdx
		currentInputIdx++
	}

	if meta.HasAudio && audioPath != "" {
		args = append(args, "-i", audioPath)
		audioIdx = currentInputIdx
		currentInputIdx++
	}

	// Filter complex building
	var filterComplex []string
	currentVideoVar := "0:v"

	// 1. Overlay Logo
	if meta.HasLogo && logoIdx != -1 {
		logoY := "20"
		if meta.LogoPosition == "bottom-center" {
			logoY = "H-h-20"
		}
		// Scale logo to logoScale% of video width, maintain aspect ratio
		scalePct := 0.15 // default
		if meta.LogoScale >= 10 && meta.LogoScale <= 100 {
			scalePct = float64(meta.LogoScale) / 100.0
		}
		scaledLogoWidth := int(float64(mainWidth) * scalePct)
		if scaledLogoWidth <= 0 {
			scaledLogoWidth = int(float64(mainWidth) * 0.15)
		}
		filterComplex = append(filterComplex, fmt.Sprintf("[%d:v]scale=w=%d:h=-1[scaled_logo]", logoIdx, scaledLogoWidth))
		filterComplex = append(filterComplex, fmt.Sprintf("[%s][scaled_logo]overlay=x=(W-w)/2:y=%s[logo_overlay]", currentVideoVar, logoY))
		currentVideoVar = "logo_overlay"
	}

	// 2. Overlay Text Hook
	if meta.Text != "" {
		wrappedText := wordWrap(meta.Text, 15) // word-wrap at ~15 characters
		
		// Create a temporary file for the text to avoid escaping and newline issues in ffmpeg
		tmpFile, err := os.CreateTemp("", "ffmpeg_text_*.txt")
		if err != nil {
			return fmt.Errorf("failed to create temp file for text: %w", err)
		}
		defer os.Remove(tmpFile.Name())
		
		if _, err := tmpFile.WriteString(wrappedText); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to write text to temp file: %w", err)
		}
		tmpFile.Close()

		fontPath := getFontPath()
		fontOption := ""
		if fontPath != "" {
			escapedFontPath := escapeFFmpegFilterPath(fontPath)
			fontOption = fmt.Sprintf("fontfile='%s':", escapedFontPath)
		}

		var textStyleParams string
		switch meta.TextStyle {
		case "neon":
			// Bright color with thick contrasting border to simulate glow
			textStyleParams = "fontcolor=0xFF00FF:fontsize=w/15:borderw=4:bordercolor=white"
		case "typewriter":
			// Dark background box
			textStyleParams = "fontcolor=white:fontsize=w/18:box=1:boxcolor=0x000000@0.7:boxborderw=15"
		default: // "classic"
			textStyleParams = "fontcolor=white:fontsize=w/15:borderw=3:bordercolor=black"
		}

		yPos := "(h-text_h)/4" // upper-third
		switch meta.TextPosition {
		case "center":
			yPos = "(h-text_h)/2"
		case "lower-third":
			yPos = "(h-text_h)*3/4"
		}

		escapedTextfilePath := escapeFFmpegFilterPath(tmpFile.Name())
		filterComplex = append(filterComplex, fmt.Sprintf("[%s]drawtext=%s%s:x=(w-text_w)/2:y=%s:textfile='%s'[text_overlay]", currentVideoVar, fontOption, textStyleParams, yPos, escapedTextfilePath))
		currentVideoVar = "text_overlay"
	}

	// 3. Burn-in Subtitles
	if meta.HasSubtitles && subtitlePath != "" {
		// Normalize subtitle file to fix missing blank lines between blocks which causes text concatenation
		content, err := os.ReadFile(subtitlePath)
		if err == nil {
			text := string(content)
			text = strings.ReplaceAll(text, "\r\n", "\n")
			text = strings.ReplaceAll(text, "\r", "\n")

			// Fix missing blank line before SRT block (Text \n Index \n Timestamp)
			reSRT := regexp.MustCompile(`([^\n])\n(\d+)\n(\d{2}:\d{2}:\d{2}[,\.]\d{3}\s*-->)`)
			text = reSRT.ReplaceAllString(text, "$1\n\n$2\n$3")

			// Fix missing blank line before VTT block (Text \n Timestamp)
			reVTT := regexp.MustCompile(`([^\n])\n(\d{2}:\d{2}:\d{2}[,\.]\d{3}\s*-->)`)
			text = reVTT.ReplaceAllString(text, "$1\n\n$2")
			
			// Fix missing blank line before VTT short block (Text \n MM:SS.mmm)
			reVTTShort := regexp.MustCompile(`([^\n])\n(\d{2}:\d{2}[,\.]\d{3}\s*-->)`)
			text = reVTTShort.ReplaceAllString(text, "$1\n\n$2")

			_ = os.WriteFile(subtitlePath, []byte(text), 0644)
		}

		escapedSubPath := escapeFFmpegFilterPath(subtitlePath)
		filterComplex = append(filterComplex, fmt.Sprintf("[%s]subtitles='%s'[subtitles_overlay]", currentVideoVar, escapedSubPath))
		currentVideoVar = "subtitles_overlay"
	}

	// 4. Audio Mixing / Muting
	var hasOutputAudio bool
	if meta.HasAudio && audioIdx != -1 {
		hasOutputAudio = true
		if !meta.MuteAudio && hasOrigAudio {
			// Mix original audio and uploaded audio
			filterComplex = append(filterComplex, fmt.Sprintf("[0:a][%d:a]amix=inputs=2:duration=first[mixed_audio]", audioIdx))
		}
	} else {
		// No custom audio
		if meta.MuteAudio {
			hasOutputAudio = false
		} else if hasOrigAudio {
			hasOutputAudio = true
		}
	}

	// Combine filterComplex if any
	var filterComplexStr string
	if len(filterComplex) > 0 {
		filterComplexStr = strings.Join(filterComplex, ";")
		args = append(args, "-filter_complex", filterComplexStr)
	}

	// Map video output
	if filterComplexStr != "" && strings.Contains(currentVideoVar, "_overlay") {
		args = append(args, "-map", "["+currentVideoVar+"]")
	} else {
		args = append(args, "-map", "0:v")
	}

	// Map audio output
	if hasOutputAudio {
		if filterComplexStr != "" && strings.Contains(filterComplexStr, "mixed_audio") {
			args = append(args, "-map", "[mixed_audio]")
		} else {
			// Fallback to mapping input audio directly
			if meta.HasAudio && audioIdx != -1 {
				args = append(args, "-map", fmt.Sprintf("%d:a", audioIdx))
			} else {
				args = append(args, "-map", "0:a")
			}
		}
		args = append(args, "-c:a", "aac")
	}

	// Video codec settings
	// Use libx264, set profile, and fast presets to speed up processing on VPS
	// Added -profile:v main and -movflags +faststart to ensure Instagram API can process and stream it quickly without 2207082 errors.
	args = append(args, "-c:v", "libx264", "-profile:v", "main", "-pix_fmt", "yuv420p", "-preset", "veryfast", "-crf", "23", "-movflags", "+faststart")

	// Cut to video duration
	args = append(args, "-t", fmt.Sprintf("%.2f", duration))

	// Output file
	args = append(args, outputPath)

	// Run command
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg process error: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

func GenerateThumbnail(ctx context.Context, videoPath, outputPath string) error {
	// Seek to 1s to avoid potential black frames at start of video.
	// If the video is shorter than 1s, we will fallback to 0s.
	args := []string{
		"-y",
		"-ss", "00:00:01",
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Fallback to start of the video (0s)
		argsFallback := []string{
			"-y",
			"-i", videoPath,
			"-vframes", "1",
			"-q:v", "2",
			outputPath,
		}
		cmdFallback := exec.CommandContext(ctx, "ffmpeg", argsFallback...)
		var stderrFallback bytes.Buffer
		cmdFallback.Stderr = &stderrFallback
		if errFallback := cmdFallback.Run(); errFallback != nil {
			return fmt.Errorf("ffmpeg thumbnail extraction failed: %w, stderr: %s", errFallback, stderrFallback.String())
		}
	}

	return nil
}

