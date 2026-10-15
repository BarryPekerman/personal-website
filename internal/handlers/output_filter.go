package handlers

import (
	"io"
	"strings"
	
	"github.com/sirupsen/logrus"
)

// filterInitialOutput filters out unwanted initial output from shell startup
// This includes sudo messages, terminal title sequences, box-drawing characters, etc.
func filterInitialOutput(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	
	// Remove all ANSI escape sequences (including OSC sequences for terminal title)
	// This handles: \x1b]0;...\x1b\\, \x1b]7;...\x1b\\, and other escape sequences
	result := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		if data[i] == 0x1b { // ESC character
			// Skip ESC and everything until BEL (\x07) or backslash (\x5c) or ST (\x1b\)
			j := i + 1
			for j < len(data) {
				if data[j] == 0x07 || data[j] == 0x5c || (j+1 < len(data) && data[j] == 0x1b && data[j+1] == '\\') {
					i = j + 1
					if data[j] == 0x1b {
						i++ // Skip the backslash too
					}
					break
				}
				j++
			}
			if j >= len(data) {
				break // Incomplete sequence, skip rest
			}
			continue
		}
		result = append(result, data[i])
		i++
	}
	
	text := string(result)
	
	// Remove sudo message (with both \r\n and \n line endings)
	text = strings.ReplaceAll(text, "To run a command as administrator (user \"root\"), use \"sudo <command>\".\r\n", "")
	text = strings.ReplaceAll(text, "To run a command as administrator (user \"root\"), use \"sudo <command>\".\n", "")
	text = strings.ReplaceAll(text, "See \"man sudo_root\" for details.\r\n", "")
	text = strings.ReplaceAll(text, "See \"man sudo_root\" for details.\n", "")
	
	// Remove box-drawing characters that might render as "W"
	// Common box-drawing chars: ╔ ═ ╗ ║ ╚ ╝ (U+2550-U+256C)
	// These might render as "W" if font doesn't support them
	text = strings.ReplaceAll(text, "╔", "")
	text = strings.ReplaceAll(text, "═", "")
	text = strings.ReplaceAll(text, "╗", "")
	text = strings.ReplaceAll(text, "║", "")
	text = strings.ReplaceAll(text, "╚", "")
	text = strings.ReplaceAll(text, "╝", "")
	
	// Remove any sequences of repeated 'W' characters at the START (likely rendering artifacts)
	// Only remove if it's at the very beginning, to avoid breaking legitimate content
	if len(text) > 0 && text[0] == 'W' {
		// Count leading W's
		wCount := 0
		for wCount < len(text) && text[wCount] == 'W' {
			wCount++
		}
		// If it's a long sequence of W's (more than 5), remove them
		// But preserve any newlines that might follow
		if wCount > 5 {
			// Check if there's a newline after the W's
			if wCount < len(text) && (text[wCount] == '\n' || text[wCount] == '\r') {
				// Keep the newline
				text = text[wCount:]
			} else {
				// No newline, just remove the W's
				text = text[wCount:]
			}
		}
	}
	
	return []byte(text)
}

// filteredReader wraps an io.Reader and filters initial unwanted output
type filteredReader struct {
	reader     io.Reader
	buffer     []byte
	filtered   bool
	skipCount  int
	logger     *logrus.Logger
}

func newFilteredReader(reader io.Reader, logger *logrus.Logger) *filteredReader {
	return &filteredReader{
		reader:   reader,
		buffer:   make([]byte, 4096),
		filtered: false,
		skipCount: 0,
		logger:   logger,
	}
}

func (fr *filteredReader) Read(p []byte) (n int, err error) {
	n, err = fr.reader.Read(fr.buffer)
	if n > 0 {
		var filtered []byte
		
		// ALWAYS filter ANSI escape sequences and unwanted output
		// The filter function handles both initial and ongoing output
		filtered = filterInitialOutput(fr.buffer[:n])
		
		// Debug: log first few reads to see what we're getting
		if fr.skipCount < 5 {
			fr.logger.WithField("skipCount", fr.skipCount).Debugf("Read %d bytes -> filtered to %d bytes", n, len(filtered))
			fr.logger.Debugf("Raw hex (first 200 bytes): %x", fr.buffer[:min(n, 200)])
			fr.logger.Debugf("Raw content (first 200 bytes): %q", string(fr.buffer[:min(n, 200)]))
			if len(filtered) > 0 {
				fr.logger.Debugf("Filtered hex (first 200 bytes): %x", filtered[:min(len(filtered), 200)])
				fr.logger.Debugf("Filtered content (first 200 bytes): %q", string(filtered[:min(len(filtered), 200)]))
			}
		}
		
		// For initial reads (first 10), skip empty filtered output
		// After that, pass through even if empty (might be just newlines)
		if fr.skipCount < 10 && len(filtered) == 0 {
			fr.skipCount++
			return fr.Read(p)
		}
		
		// Copy filtered data
		copyLen := len(filtered)
		if copyLen > len(p) {
			copyLen = len(p)
		}
		if copyLen > 0 {
			copy(p, filtered[:copyLen])
		}
		fr.skipCount++
		return copyLen, err
	}
	return n, err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

