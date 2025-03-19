package librealsense2

/*
#cgo linux darwin LDFLAGS: -L/usr/local/lib/ -lrealsense2
#cgo CPPFLAGS: -I/usr/local/include
#include <librealsense2/rs.h>
#include <librealsense2/h/rs_pipeline.h>
*/
import "C"

import (
	"time"
	"unsafe"
)

// FilterType is a type of filter.
type FilterType int

const (
	Decimation FilterType = iota
	Threshold
	DepthDisparity
	Spatial
	Temporal
	HoleFilling
	DisparityToDepth
	RatesPrinter
)

// Filter Librealsense implementation includes post-processing filters to enhance
// the quality of depth data and reduce noise levels. All the filters are
// implemented in the library core as independent blocks to be used in the
// customer code
type Filter struct {
	typ     FilterType
	options map[int]float64

	queue  *C.rs2_frame_queue
	filter *C.rs2_processing_block
}

// NewFilter creates a new filter of the given type.
func NewFilter(t FilterType) *Filter {
	return &Filter{typ: t}
}

func (f *Filter) initialize() error {
	var errc *C.rs2_error

	f.queue = C.rs2_create_frame_queue(1, &errc)
	if errc != nil {
		return errorFrom(errc)
	}

	switch f.typ {
	case Decimation:
		f.filter = C.rs2_create_decimation_filter_block(&errc)
	case Spatial:
		f.filter = C.rs2_create_spatial_filter_block(&errc)
	case Threshold:
		f.filter = C.rs2_create_threshold(&errc)
	case DisparityToDepth:
		f.filter = C.rs2_create_disparity_transform_block(0, &errc)
	case DepthDisparity:
		f.filter = C.rs2_create_disparity_transform_block(1, &errc)
	case RatesPrinter:
		f.filter = C.rs2_create_rates_printer_block(&errc)
	case Temporal:
		f.filter = C.rs2_create_temporal_filter_block(&errc)
	case HoleFilling:
		f.filter = C.rs2_create_hole_filling_filter_block(&errc)
	}

	if errc != nil {
		return errorFrom(errc)
	}

	for opt, value := range f.options {
		C.rs2_set_option((*C.rs2_options)(unsafe.Pointer(f.filter)), C.rs2_option(opt), C.float(value), &errc)
		if errc != nil {
			return errorFrom(errc)
		}
	}

	C.rs2_start_processing_queue(f.filter, f.queue, &errc)
	if errc != nil {
		return errorFrom(errc)
	}

	return nil
}

// Apply applies the filter to the given frame.
func (f *Filter) Apply(frame *C.rs2_frame, timeout time.Duration) (*C.rs2_frame, error) {
	if timeout == 0 {
		timeout = defaultTimeout
	}

	var errc *C.rs2_error
	C.rs2_process_frame(f.filter, frame, &errc)
	if errc != nil {
		return nil, errorFrom(errc)
	}

	ms := timeout.Milliseconds()
	result := C.rs2_wait_for_frame(f.queue, C.uint(ms), &errc)
	if errc != nil {
		return nil, errorFrom(errc)
	}

	return result, nil
}

// Close closes the filter.
func (f *Filter) Close() error {
	C.rs2_delete_processing_block(f.filter)
	C.rs2_delete_frame_queue(f.queue)
	return nil
}

// ThresholdFilter filters by min and max distance, in meters.
func ThresholdFilter(min, max float64) *Filter {
	return &Filter{
		typ: Threshold,
		options: map[int]float64{
			C.RS2_OPTION_MIN_DISTANCE: min,
			C.RS2_OPTION_MAX_DISTANCE: max,
		},
	}
}

// DisparityToDepthFilter if we are in disparity domain, switch back to depth.
func DisparityToDepthFilter() *Filter {
	return &Filter{
		typ: DisparityToDepth,
	}
}

// DepthDisparityFilter change to disparity domain.
func DepthDisparityFilter() *Filter {
	return &Filter{
		typ: DepthDisparity,
	}
}

// DefaultTemporalFilter is the default temporal filter used by the pipeline.
func DefaultTemporalFilter() *Filter {
	return TemporalFilter(0.4, 20, 3)
}

// TemporalFilter is intended to improve the depth data persistency by
// manipulating per-pixel values based on previous frames. The filter performs
// a single pass on the data, adjusting the depth values while also updating the
// tracking history. In cases where the pixel data is missing or invalid, the
// filter uses a user-defined persistency mode to decide whether the missing
// value should be rectified with stored data. Note that due to its reliance on
// historic data the filter may introduce visible blurring/smearing artifacts,
// and therefore is best-suited for static scenes.
//
// Persistency modes:
// 0 - Disabled - The Persistency filter is not activated and no hole filling occurs.
// 1 - Valid in 8/8 - Persistency activated if the pixel was valid in 8 out of the last 8 frames
// 2 - Valid in 2/last 3 - Activated if the pixel was valid in two out of the last 3 frames
// 3 - Valid in 2/last 4 - Activated if the pixel was valid in two out of the last 4 frames
// 4 - Valid in 2/8 - Activated if the pixel was valid in two out of the last 8 frames
// 5 - Valid in 1/last 2 - Activated if the pixel was valid in one of the last two frames
// 6 - Valid in 1/last 5 - Activated if the pixel was valid in one out of the last 5 frames
// 7 - Valid in 1/last 8 - Activated if the pixel was valid in one out of the last 8 frames
// 8 - Persist Indefinitely - Persistency will be imposed regardless of the stored history (most aggressive filtering)
func TemporalFilter(smoothAlpha, smoothDelta, persistency float64) *Filter {
	if smoothAlpha < 0 || smoothAlpha > 1 {
		panic("smoothAlpha must be between 0 and 1")
	}

	if smoothDelta < 1 || smoothDelta > 100 {
		panic("smoothDelta must be between 1 and 100")
	}

	if persistency < 0 || persistency > 8 {
		panic("persistency must be between 0 and 8")
	}

	return &Filter{
		typ: Temporal,
		options: map[int]float64{
			C.RS2_OPTION_FILTER_SMOOTH_ALPHA: smoothAlpha,
			C.RS2_OPTION_FILTER_SMOOTH_DELTA: smoothDelta,
			C.RS2_OPTION_HOLES_FILL:          persistency,
		},
	}
}

// DefaultDecimationFilter is a decimatin filter with default values.
func DefaultDecimationFilter() *Filter {
	return DecimationFilter(2)
}

// DefaultDecimationFilter effectively reduces the depth scene complexity. The
// filter runs on kernel sizes [2x2] to [8x8] pixels. For patches sized 2 and 3
// the median depth value is selected. For larger kernels, 4-8 pixels, the mean
// depth is used due to performance considerations.
func DecimationFilter(magnitude float64) *Filter {
	return &Filter{
		typ: Decimation,
		options: map[int]float64{
			C.RS2_OPTION_FILTER_MAGNITUDE: magnitude,
		},
	}
}

// DefaultHoleFillingFilter is a hole filling filter with default values.
func DefaultHoleFillingFilter() *Filter {
	return HoleFillingFilter(1)
}

// HoleFillingFilter implements several methods to rectify missing data in the
// resulting image. The filter obtains the four immediate pixel "neighbors"
// (up, down, left, right), and selects one of them according to a user-defined
// rule.
//
// Mode:
// 0 - fill_from_left - Use the value from the left neighbor pixel to fill the hole
// 1 - farest_from_around - Use the value from the neighboring pixel which is furthest away from the sensor
// 2 - nearest_from_around  - Use the value from the neighboring pixel closest to the sensor
func HoleFillingFilter(mode float64) *Filter {
	if mode < 0 || mode > 2 {
		panic("mode must be between 0 and 2")
	}

	return &Filter{
		typ: HoleFilling,
		options: map[int]float64{
			C.RS2_OPTION_HOLES_FILL: mode,
		},
	}
}

// DefaultSpatialFilter is a spatial filter with default values.
func DefaultSpatialFilter() *Filter {
	return SpatialFilter(2, 0.50, 20)
}

// SpatialFilter performs a series of 1D horizontal and vertical passes or
// iterations, to enhance the smoothness of the reconstructed data.
func SpatialFilter(magnitude, smoothAlpha, smoothDelta float64) *Filter {
	if magnitude < 1 || magnitude > 5 {
		panic("magnitude must be between 1 and 5")
	}

	if smoothAlpha < 0.25 || smoothAlpha > 1 {
		panic("smoothAlpha must be between 0.25 and 1")
	}

	if smoothDelta < 1 || smoothDelta > 50 {
		panic("smoothDelta must be between 1 and 50")
	}

	return &Filter{
		typ: Spatial,
		options: map[int]float64{
			C.RS2_OPTION_FILTER_SMOOTH_ALPHA: smoothAlpha,
			C.RS2_OPTION_FILTER_SMOOTH_DELTA: smoothDelta,
			C.RS2_OPTION_FILTER_MAGNITUDE:    magnitude,
		},
	}
}
