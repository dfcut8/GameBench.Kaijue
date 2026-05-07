package main

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
)

const (
	sampleRate = 44100
	duration   = 8
	channels   = 1
	bitDepth   = 16
)

func main() {
	outDir := filepath.Join("assets", "audio")
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		panic(err)
	}
	path := filepath.Join(outDir, "benchmark_loop.wav")
	data := makeMusic()
	if err := os.WriteFile(path, data, os.ModePerm); err != nil {
		panic(err)
	}
}

func makeMusic() []byte {
	totalSamples := sampleRate * duration
	pcmBytes := totalSamples * channels * bitDepth / 8
	out := make([]byte, 44+pcmBytes)

	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(36+pcmBytes))
	copy(out[8:], "WAVE")
	copy(out[12:], "fmt ")
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 1)
	binary.LittleEndian.PutUint16(out[22:], channels)
	binary.LittleEndian.PutUint32(out[24:], sampleRate)
	binary.LittleEndian.PutUint32(out[28:], sampleRate*channels*bitDepth/8)
	binary.LittleEndian.PutUint16(out[32:], channels*bitDepth/8)
	binary.LittleEndian.PutUint16(out[34:], bitDepth)
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(pcmBytes))

	scale := []int{0, 3, 5, 7, 10, 7, 5, 3}
	bass := []float64{110, 110, 146.83, 146.83, 164.81, 146.83, 130.81, 130.81}
	for i := 0; i < totalSamples; i++ {
		t := float64(i) / sampleRate
		step := int(t*4) % len(scale)
		freq := 220 * math.Pow(2, float64(scale[step])/12)
		beat := int(t*8) % 2
		gate := 0.35
		if beat == 0 {
			gate = 0.55
		}
		env := 1.0
		if math.Mod(t*4, 1) > gate {
			env = 0
		}
		lead := square(freq, t) * 0.18 * env
		low := square(bass[int(t)%len(bass)], t) * 0.09
		click := 0.0
		if math.Mod(t*8, 1) < 0.035 {
			click = 0.08 * math.Sin(2*math.Pi*880*t)
		}
		sample := math.Tanh(lead + low + click)
		binary.LittleEndian.PutUint16(out[44+i*2:], uint16(int16(sample*24000)))
	}
	return out
}

func square(freq, t float64) float64 {
	if math.Sin(2*math.Pi*freq*t) >= 0 {
		return 1
	}
	return -1
}
