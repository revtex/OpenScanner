// Package stream serves listeners a single, continuous audio stream
// instead of one HTTP request per call.
//
// Why this exists: iOS only keeps a backgrounded page alive while audio is
// actively playing. A scanner is silent between calls, so a phone with the
// screen locked gets suspended in the gap and never plays the next call.
// Feeding the browser one never-ending <audio> source — silence padded
// with calls as they arrive — keeps the audio session open, which is how
// native scanner apps and Broadcastify behave.
//
// The stream is assembled by concatenating MPEG audio frames rather than
// by running an encoder per listener: every call is transcoded once into
// the canonical format below, and the resulting frames are handed to every
// listener that should hear it. That only works because each frame is
// self-contained, which is why the encoder is invoked with `-reservoir 0`
// (the bit reservoir would otherwise make a frame depend on its
// predecessors and produce artefacts at every splice).
package stream

// Canonical stream format. Every source — silence and calls alike — is
// transcoded to exactly these parameters, because frames can only be
// concatenated when their sample rate, channel count and bitrate agree.
// 22.05 kHz mono at 32 kbps is MPEG-2 Layer III: 576 samples per frame,
// ~26.1 ms of audio in ~105 bytes, which is ample for voice traffic and
// cheap enough to leave running on a phone.
const (
	streamSampleRate = 22050
	streamBitrate    = "32k"
	streamChannels   = 1
	// ContentType is the MIME type the HTTP handler advertises.
	ContentType = "audio/mpeg"
)

// Layer III bitrate tables, in kbps, indexed by the header's bitrate bits.
// Index 0 ("free") and 15 ("bad") are invalid and stored as zero.
var (
	bitratesV1 = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	bitratesV2 = [16]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}

	sampleRatesV1  = [4]int{44100, 48000, 32000, 0}
	sampleRatesV2  = [4]int{22050, 24000, 16000, 0}
	sampleRatesV25 = [4]int{11025, 12000, 8000, 0}
)

// mpegFrame is the decoded form of a 4-byte MPEG audio frame header.
type mpegFrame struct {
	length     int // whole frame in bytes, header included
	sampleRate int
	samples    int // PCM samples the frame decodes to
	channels   int
}

// parseFrameHeader decodes an MPEG Layer III frame header. It reports
// false for anything that is not a valid Layer III frame, which is how
// splitFrames resynchronises after tags or padding.
func parseFrameHeader(b []byte) (mpegFrame, bool) {
	if len(b) < 4 {
		return mpegFrame{}, false
	}
	// 11-bit frame sync.
	if b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return mpegFrame{}, false
	}

	version := (b[1] >> 3) & 0x03 // 0=MPEG2.5, 1=reserved, 2=MPEG2, 3=MPEG1
	layer := (b[1] >> 1) & 0x03   // 1 = Layer III
	if version == 1 || layer != 1 {
		return mpegFrame{}, false
	}

	bitrateIdx := (b[2] >> 4) & 0x0F
	rateIdx := (b[2] >> 2) & 0x03
	padding := int((b[2] >> 1) & 0x01)
	chanMode := (b[3] >> 6) & 0x03
	if bitrateIdx == 0 || bitrateIdx == 15 || rateIdx == 3 {
		return mpegFrame{}, false
	}

	var kbps, rate, samples, coefficient int
	switch version {
	case 3: // MPEG 1
		kbps, rate = bitratesV1[bitrateIdx], sampleRatesV1[rateIdx]
		samples, coefficient = 1152, 144
	case 2: // MPEG 2
		kbps, rate = bitratesV2[bitrateIdx], sampleRatesV2[rateIdx]
		samples, coefficient = 576, 72
	default: // MPEG 2.5
		kbps, rate = bitratesV2[bitrateIdx], sampleRatesV25[rateIdx]
		samples, coefficient = 576, 72
	}
	if kbps == 0 || rate == 0 {
		return mpegFrame{}, false
	}

	channels := 2
	if chanMode == 3 { // single channel
		channels = 1
	}

	length := coefficient*kbps*1000/rate + padding
	if length < 4 {
		return mpegFrame{}, false
	}
	return mpegFrame{
		length:     length,
		sampleRate: rate,
		samples:    samples,
		channels:   channels,
	}, true
}

// splitFrames carves an encoder's output into whole MPEG frames, skipping
// a leading ID3v2 tag and any bytes that do not parse as a frame header. A
// trailing partial frame is dropped rather than emitted, since a listener
// must only ever be handed complete frames.
func splitFrames(data []byte) [][]byte {
	i := 0
	if len(data) >= 10 && string(data[0:3]) == "ID3" {
		// Syncsafe integer: 7 significant bits per byte.
		size := int(data[6]&0x7F)<<21 | int(data[7]&0x7F)<<14 |
			int(data[8]&0x7F)<<7 | int(data[9]&0x7F)
		i = 10 + size
	}

	var frames [][]byte
	for i < len(data) {
		f, ok := parseFrameHeader(data[i:])
		if !ok {
			i++
			continue
		}
		if i+f.length > len(data) {
			break
		}
		frames = append(frames, data[i:i+f.length])
		i += f.length
	}
	return frames
}

// frameDuration reports how much audio a parsed frame carries. Returns
// zero for a frame that does not parse, so callers can treat it as
// "unknown" rather than dividing by zero.
func frameDuration(frame []byte) (samples, rate int) {
	f, ok := parseFrameHeader(frame)
	if !ok {
		return 0, 0
	}
	return f.samples, f.sampleRate
}
