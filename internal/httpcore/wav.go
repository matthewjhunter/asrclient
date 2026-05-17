// Package httpcore implements the shared HTTP transport for the
// OpenAI-compatible /v1/audio/transcriptions endpoint, used by both the
// openai and whispercpp Backends.
package httpcore

import "encoding/binary"

// PCMToWav wraps int16-LE PCM in a 44-byte canonical RIFF/WAVE header.
// The sample rate, channel count, and sample width are fixed to
// asrclient's locked format (16 kHz mono int16-LE) so callers do not
// need to thread format parameters through.
func PCMToWav(pcm []byte) []byte {
	const (
		sampleRate    = 16000
		numChannels   = 1
		bitsPerSample = 16
	)
	byteRate := uint32(sampleRate * numChannels * bitsPerSample / 8)
	blockAlign := uint16(numChannels * bitsPerSample / 8)
	// WAV's data-size field is 32-bit; 4 GiB of int16 PCM at 16 kHz mono
	// is ~34 hours per call, well outside any realistic Transcribe use.
	dataSize := uint32(len(pcm)) //nolint:gosec // see comment above

	buf := make([]byte, 44+len(pcm))
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], 36+dataSize)
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16) // PCM fmt chunk size
	binary.LittleEndian.PutUint16(buf[20:22], 1)  // PCM format
	binary.LittleEndian.PutUint16(buf[22:24], numChannels)
	binary.LittleEndian.PutUint32(buf[24:28], sampleRate)
	binary.LittleEndian.PutUint32(buf[28:32], byteRate)
	binary.LittleEndian.PutUint16(buf[32:34], blockAlign)
	binary.LittleEndian.PutUint16(buf[34:36], bitsPerSample)
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], dataSize)
	copy(buf[44:], pcm)
	return buf
}
