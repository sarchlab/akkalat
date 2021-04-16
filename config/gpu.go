package config

import (
	"gitlab.com/akita/akita/monitoring"
	"gitlab.com/akita/akita/v2/sim"
)

// A WaferScaleGPUBuilder can build wafer-scale GPUs.
type WaferScaleGPUBuilder struct {
	engine  sim.Engine
	freq    sim.Freq
	monitor *monitoring.Monitor

	tileWidth    int
	tileHeight   int
	numCUPerTile int
}

// MakeWaferScaleGPUBuilder creates a WaferScaleGPUBuilder.
func MakeWaferScaleGPUBuilder() WaferScaleGPUBuilder {
	return WaferScaleGPUBuilder{
		freq:       sim.GHz,
		tileWidth:  25,
		tileHeight: 25,
	}
}

// WithEngine sets the engine that the GPU to use.
func (b WaferScaleGPUBuilder) WithEngine(e sim.Engine) WaferScaleGPUBuilder {
	b.engine = e
	return b
}

// WithFreq sets the frequency that the wafer-scale GPU works at.
func (b WaferScaleGPUBuilder) WithFreq(f sim.Freq) WaferScaleGPUBuilder {
	b.freq = f
	return b
}

// WithMonitor sets the monitor that monitoring the execution of the GPU's
// internal components.
func (b WaferScaleGPUBuilder) WithMonitor(
	m *monitoring.Monitor,
) WaferScaleGPUBuilder {
	b.monitor = m
	return b
}

// WithTileWidth sets the number of tiles horizontally.
func (b WaferScaleGPUBuilder) WithTileWidth(w int) WaferScaleGPUBuilder {
	b.tileWidth = w
	return b
}

// WithTileHeight sets the number of tiles vertically.
func (b WaferScaleGPUBuilder) WithTileHeight(h int) WaferScaleGPUBuilder {
	b.tileHeight = h
	return b
}

// Build builds a new wafer-scale GPU.
func (b WaferScaleGPUBuilder) Build(name string) *GPU {
	return &GPU{}
}
