package librealsense2

/*
#cgo linux darwin LDFLAGS: -L/usr/local/lib/ -lrealsense2
#cgo CPPFLAGS: -I/usr/local/include
#include <librealsense2/rs.h>
#include <librealsense2/h/rs_pipeline.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"gocv.io/x/gocv"
)

const defaultTimeout = time.Second

var (
	ErrNotDevicesFound = errors.New("no realsense devices found")
)

// Stream is a RealSense data stream.
type Stream C.rs2_stream

const (
	StreamDepth Stream = C.RS2_STREAM_DEPTH
	StreamColor Stream = C.RS2_STREAM_COLOR
)

// Pipepline represents a realsense pipeline, includes a device and a selection
// of active streams.
type Pipeline struct {
	p       *C.rs2_pipeline
	ctx     *C.rs2_context
	conf    *C.rs2_config
	profile *C.rs2_pipeline_profile

	filters []*Filter
}

// NewPipeline creates a new pipeline with the given serial number. If serial is
// empty, the first device will be used.
func NewPipeline(serial string) (*Pipeline, error) {
	var err *C.rs2_error
	ctx := C.rs2_create_context(C.RS2_API_VERSION, &err)
	if err != nil {
		return nil, errorFrom(err)
	}

	device_list := C.rs2_query_devices(ctx, &err)
	if err != nil {
		return nil, errorFrom(err)
	}

	dev_count := C.rs2_get_device_count(device_list, &err)
	if err != nil {
		return nil, errorFrom(err)
	}

	if C.int(dev_count) == 0 {
		return nil, ErrNotDevicesFound
	}

	conf := C.rs2_create_config(&err)
	if err != nil {
		return nil, errorFrom(err)
	}

	if serial != "" {
		for i := 0; i < int(dev_count); i++ {
			dev := C.rs2_create_device(device_list, C.int(i), &err)
			if err != nil {
				fmt.Println(errorFrom(err))
				continue
			}

			s := C.rs2_get_device_info(dev, C.RS2_CAMERA_INFO_SERIAL_NUMBER, &err)
			if C.GoString(s) == serial {
				if C.rs2_config_enable_device(conf, s, &err); err != nil {
					return nil, errorFrom(err)
				}
			}

			C.rs2_delete_device(dev)
		}
	}

	p := C.rs2_create_pipeline(ctx, &err)
	if err != nil {
		return nil, errorFrom(err)
	}

	C.rs2_delete_device_list(device_list)

	pl := &Pipeline{
		p:    p,
		ctx:  ctx,
		conf: conf,
	}

	return pl, nil
}

// EnableStream enables a stream with the given width, height and fps.
func (p *Pipeline) EnableStream(stream Stream, width, height, fps int) error {
	var format C.rs2_format
	switch stream {
	case StreamDepth:
		format = C.RS2_FORMAT_Z16
	case StreamColor:
		format = C.RS2_FORMAT_BGR8
	default:
		return fmt.Errorf("unknown stream %d", stream)
	}

	var err *C.rs2_error
	C.rs2_config_enable_stream(p.conf, C.rs2_stream(stream), 0, C.int(width), C.int(height), format, C.int(fps), &err)

	if err != nil {
		return errorFrom(err)
	}

	return nil
}

// SetDefaultPostProcessingFilters sets the default post-processing filters.
func (p *Pipeline) SetDefaultPostProcessingFilters() {
	p.SetPostProcessingFilters(
		DefaultDecimationFilter(),
		DepthDisparityFilter(),
		DefaultSpatialFilter(),
		DefaultTemporalFilter(),
		DefaultHoleFillingFilter(),
		DisparityToDepthFilter(),
	)
}

// SetPostProcessingFilters sets the post-processing filters to be applied to the
// frames. The post-processing filters are designed and built for
// concatenation into processing pipes. There are no software-imposed constraints
// that mandate the order in which the filters shall be applied. At the same
// time the recommended scheme used in librealsense tools and demos is elaborated
// below:
//
// Depth Frame >> Decimation Filter >> Depth2Disparity Transform** >>
// Spatial Filter >> Temporal Filter >> Disparity2Depth Transform** >>
// Hole Filling Filter >> Filtered Depth.
func (p *Pipeline) SetPostProcessingFilters(f ...*Filter) {
	p.filters = f
}

// Start starts the pipeline.
func (p *Pipeline) Start() error {
	for _, f := range p.filters {
		f.initialize()
	}

	var err *C.rs2_error
	prof := C.rs2_pipeline_start_with_config(p.p, p.conf, &err)
	if err != nil {
		return errorFrom(err)
	}

	p.profile = prof
	return nil
}

// Close closes the pipeline and frees all resources.
func (p *Pipeline) Close() error {
	var err *C.rs2_error
	C.rs2_pipeline_stop(p.p, &err)
	if err != nil {
		return errorFrom(err)
	}

	C.rs2_delete_pipeline_profile(p.profile)
	C.rs2_delete_config(p.conf)
	C.rs2_delete_pipeline(p.p)
	C.rs2_delete_context(p.ctx)

	var errs []error
	for _, f := range p.filters {
		if err := f.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// WaitFrames waits for frames from the pipeline. Color and depth frames are
// sent to the frame channel. If an error occurs, it is sent to the error channel.
// If the error channel is nil, the error is ignored.
func (p *Pipeline) WaitFrames(frameCh chan *gocv.Mat, errCh chan error, timeout time.Duration) error {
	if timeout == 0 {
		timeout = defaultTimeout
	}

	ms := timeout.Milliseconds()

	for {
		var errc *C.rs2_error
		frames := C.rs2_pipeline_wait_for_frames(p.p, C.uint(ms), &errc)
		if errc != nil {
			if errCh != nil {
				errCh <- errorFrom(errc)
			}

			continue
		}

		count := C.rs2_embedded_frames_count(frames, &errc)
		if errc != nil {
			errCh <- errorFrom(errc)
			continue
		}

		for i := 0; i < int(count); i++ {
			var ret gocv.Mat
			var err error

			frame := C.rs2_extract_frame(frames, C.int(i), &errc)
			if frame == nil {
				continue
			}

			if C.rs2_is_frame_extendable_to(frame, C.RS2_EXTENSION_DEPTH_FRAME, &errc) != 0 {
				for _, f := range p.filters {
					filtered, err := f.Apply(frame, 0)
					if err != nil {
						if errCh != nil {
							errCh <- errorFrom(errc)
						}
					}

					frame = filtered
				}
			}

			w := C.rs2_get_frame_width(frame, &errc)
			h := C.rs2_get_frame_height(frame, &errc)

			t := gocv.MatTypeCV8UC3
			sz := w * h * 3
			if C.rs2_is_frame_extendable_to(frame, C.RS2_EXTENSION_DEPTH_FRAME, &errc) != 0 {
				t = gocv.MatTypeCV16UC1
				sz = w * h * 2
			}

			frameData := C.rs2_get_frame_data(frame, &errc)
			if errc != nil {
				if errCh != nil {
					errCh <- errorFrom(errc)
				}

				continue
			}

			b := C.GoBytes(unsafe.Pointer(frameData), C.int(sz))
			ret, err = gocv.NewMatFromBytes(int(h), int(w), t, b)
			if err != nil {
				if errCh != nil {
					errCh <- err
				}
			} else {
				frameCh <- &ret
			}

			C.rs2_release_frame(frame)
		}

		C.rs2_release_frame(frames)
	}
}

func errorFrom(err *C.rs2_error) error {
	if err == nil {
		return nil
	}

	defer C.rs2_free_error(err)
	return fmt.Errorf(C.GoString(C.rs2_get_error_message(err)))
}
