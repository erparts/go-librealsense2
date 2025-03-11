package main

import (
	"log"
	"time"

	"github.com/erparts/go-librealsense2"
	"gocv.io/x/gocv"
)

func main() {
	pipeline, err := librealsense2.NewPipeline("")
	if err != nil {
		log.Fatalf("Failed to create pipeline: %v", err)
	}
	defer pipeline.Close()

	if err := pipeline.EnableStream(librealsense2.StreamDepth, 640, 480, 30); err != nil {
		log.Fatalf("Failed to enable depth stream: %v", err)
	}

	if err := pipeline.EnableStream(librealsense2.StreamColor, 640, 480, 30); err != nil {
		log.Fatalf("Failed to enable color stream: %v", err)
	}

	if err := pipeline.Start(); err != nil {
		log.Fatalf("Failed to start pipeline: %v", err)
	}

	windowDepth := gocv.NewWindow("Depth")
	defer windowDepth.Close()
	windowColor := gocv.NewWindow("Color")
	defer windowColor.Close()

	ch := make(chan *gocv.Mat, 1)
	go func() {
		for {
			frame := <-ch
			if frame == nil {
				continue
			}

			switch frame.Type() {
			case gocv.MatTypeCV8UC3:
				windowColor.IMShow(*frame)
				windowColor.WaitKey(1)
			case gocv.MatTypeCV16UC1:
				depthImage := gocv.NewMat()
				frame.ConvertToWithParams(&depthImage, gocv.MatTypeCV8UC1, 255.0/5535.0, 0)

				coloredDepth := gocv.NewMat()
				gocv.ApplyColorMap(depthImage, &coloredDepth, gocv.ColormapJet)

				windowDepth.IMShow(coloredDepth)
				windowDepth.WaitKey(1)

				depthImage.Close()
				coloredDepth.Close()
			default:
				log.Println("Unknown frame type:", frame.Type())
			}

			frame.Close()
		}
	}()

	if err := pipeline.WaitColorFrames(ch, time.Second); err != nil {
		log.Println("Error waiting for frames:", err)
	}
}
