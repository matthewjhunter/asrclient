// Package asrclient is a unified Go API for speech-to-text transcription
// over Wyoming, the OpenAI HTTP protocol, and a local whisper.cpp HTTP
// server. The Transcriber interface is the public surface; backend
// implementations live in subpackages.
package asrclient

// Locked PCM frame format. Every Transcriber implementation in this module
// assumes audio supplied to Transcribe is encoded in this exact shape;
// callers should resample upstream.
//
// 16 kHz mono int16 little-endian. 80 ms per frame = 1280 samples =
// 2560 bytes. This matches the conventions of the Wyoming
// voice-services ecosystem and openWakeWord, so consumers that already
// produce frames in this format pay no resampling cost.
const (
	SampleRateHz = 16000
	SampleWidth  = 2
	Channels     = 1
	FrameMS      = 80
	FrameSamples = SampleRateHz * FrameMS / 1000 // 1280
	FrameBytes   = FrameSamples * SampleWidth    // 2560
)
