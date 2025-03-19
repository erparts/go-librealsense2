# go-librealsense2

## Overview

`go-librealsense2` is a Go wrapper for Intel's RealSense SDK 2.0 (`librealsense2`). It enables Go developers to interact with RealSense depth cameras, accessing features like depth sensing, RGB image capture, and point cloud generation.

## Prerequisites

- Go 1.18 or later
- Intel RealSense SDK 2.0 (`librealsense2`)
- CMake and build tools
- Compatible RealSense camera

## Installation

### 1. Install `librealsense2`

Follow the official [installation guide](https://github.com/IntelRealSense/librealsense?tab=readme-ov-file#download-and-install) to install the SDK on your system.

### 2. Install `go-librealsense2`

```sh
go get github.com/erparts/go-librealsense2/v2
```

## Usage

### Initializing a Pipeline

```go
package main

import (
	"log"
	"time"

	"github.com/erparts/go-librealsense2/v2"
	"gocv.io/x/gocv"
)

func main() {
	pipeline, err := librealsense2.NewPipeline("")
	if err != nil {
		log.Fatalf("Failed to create pipeline: %v", err)
	}
	defer pipeline.Close()

	if err := pipeline.EnableStream(librealsense2.StreamColor, 640, 480, 30); err != nil {
		log.Fatalf("failed to enable color stream: %v", err)
	}

	if err := pipeline.Start(); err != nil {
		log.Fatalf("failed to start pipeline: %v", err)
	}

	windowColor := gocv.NewWindow("Color")
	defer windowColor.Close()

	ch := make(chan *gocv.Mat, 1)
	errCh := make(chan error, 1)
	go func() {
		for {
			select {
			case err := <-errCh:
				log.Println("error waiting for frames:", err)
			case frame := <-ch:
				if frame == nil {
					continue
				}

				windowColor.IMShow(*frame)
				windowColor.WaitKey(1)

				frame.Close()
			}
		}
	}()

	if err := pipeline.WaitFrames(ch, nil, time.Second); err != nil {
		log.Println("error waiting for frames:", err)
	}
}
```

## API Reference

Refer to the documentation for a complete [API reference](https://pkg.go.dev/github.com/erparts/go-librealsense2/v2).


## License

This project is licensed under the MIT License. See `LICENSE` for details.

## Acknowledgments

- Base on the original [work](https://github.com/suutaku/go-librealsense2) by @suutaku
