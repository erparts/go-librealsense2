package main

import (
	"gocv.io/x/gocv"
)

// FilterForeground applies a depth threshold to highlight only foreground objects
func FilterForeground(depthMat gocv.Mat, minDepth, maxDepth float64) gocv.Mat {
	// Ensure the input is not empty
	if depthMat.Empty() {
		return gocv.NewMat()
	}

	// Convert depth to 8-bit grayscale for visualization
	depth8U := gocv.NewMat()
	gocv.ConvertScaleAbs(depthMat, &depth8U, 255.0/maxDepth, 0)

	// Apply threshold to keep only objects in the range [minDepth, maxDepth]
	thresholded := gocv.NewMat()
	gocv.InRangeWithScalar(depthMat, gocv.NewScalar(minDepth, 0, 0, 0), gocv.NewScalar(maxDepth, 0, 0, 0), &thresholded)

	// Apply a colormap for visualization
	coloredDepth := gocv.NewMat()
	gocv.ApplyColorMap(thresholded, &coloredDepth, gocv.ColormapJet)

	// Cleanup intermediate Mats
	depth8U.Close()
	thresholded.Close()

	return coloredDepth // Return the processed depth image
}
