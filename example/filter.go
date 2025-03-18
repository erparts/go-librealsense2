package main

import (
	"image/color"

	"gocv.io/x/gocv"
)

// FilterForeground applies a depth threshold to highlight only foreground objects, with noise reduction
func FilterForeground(depthMat gocv.Mat, minDepth, maxDepth float64) gocv.Mat {
	minContourArea := 500.0
	// Ensure the input is not empty
	if depthMat.Empty() {
		return gocv.NewMat()
	}

	// Apply threshold to keep only objects in the range [minDepth, maxDepth]
	thresholded := gocv.NewMat()
	gocv.InRangeWithScalar(depthMat, gocv.NewScalar(minDepth, 0, 0, 0), gocv.NewScalar(maxDepth, 0, 0, 0), &thresholded)

	// Find contours
	contours := gocv.FindContours(thresholded, gocv.RetrievalExternal, gocv.ChainApproxSimple)

	// Create a blank mask to draw filtered contours
	mask := gocv.Zeros(thresholded.Rows(), thresholded.Cols(), gocv.MatTypeCV8UC1)

	// Loop through contours and keep only large ones
	for i := 0; i < contours.Size(); i++ {
		area := gocv.ContourArea(contours.At(i))
		if area > minContourArea {
			// Draw the large contour on the mask
			gocv.DrawContours(&mask, contours, i, color.RGBA{255, 255, 255, 0}, -1)
		}
	}

	// Apply a colormap for better visualization
	coloredDepth := gocv.NewMat()
	gocv.ApplyColorMap(mask, &coloredDepth, gocv.ColormapJet)

	// Cleanup intermediate Mats
	thresholded.Close()
	mask.Close()

	return coloredDepth // Return the processed depth image with large contours only
}
