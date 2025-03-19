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
