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

const DefaultTimeout = time.Second * 15

var (
	ErrNotDevicesFound = errors.New("no realsense devices found")
)

type Stream C.rs2_stream

const (
	StreamDepth Stream = C.RS2_STREAM_DEPTH
	StreamColor Stream = C.RS2_STREAM_COLOR
)

type Pipeline struct {
	cfg *PipelineConfig

	p       *C.rs2_pipeline
	ctx     *C.rs2_context
	conf    *C.rs2_config
	profile *C.rs2_pipeline_profile
}

type PipelineConfig struct {
	Serial        string
	Width, Height int
	FPS           int
	DepthStream   bool
	ColorStream   bool
}

func (c *PipelineConfig) Initialize() {
	if c.Width == 0 {
		c.Width = 640
	}

	if c.Height == 0 {
		c.Height = 480
	}

	if c.FPS == 0 {
		c.FPS = 30
	}

	if !c.ColorStream && !c.DepthStream {
		c.DepthStream = true
	}
}

func NewPipeline(cfg *PipelineConfig) (*Pipeline, error) {
	cfg.Initialize()

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

	if cfg.Serial != "" {
		for i := 0; i < int(dev_count); i++ {
			dev := C.rs2_create_device(device_list, C.int(i), &err)
			if err != nil {
				fmt.Println(errorFrom(err))
				continue
			}

			s := C.rs2_get_device_info(dev, C.RS2_CAMERA_INFO_SERIAL_NUMBER, &err)
			if C.GoString(s) == cfg.Serial {
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
		cfg:  cfg,
		p:    p,
		ctx:  ctx,
		conf: conf,
	}

	return pl, nil
}

func (pl *Pipeline) EnableStream(stream Stream, width, height, fps int) error {
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
	C.rs2_config_enable_stream(pl.conf, C.rs2_stream(stream), 0, C.int(width), C.int(height), format, C.int(fps), &err)

	if err != nil {
		return errorFrom(err)
	}

	return nil
}

func (pl *Pipeline) Close() error {
	var err *C.rs2_error
	C.rs2_pipeline_stop(pl.p, &err)
	if err != nil {
		return errorFrom(err)
	}

	C.rs2_delete_pipeline_profile(pl.profile)
	C.rs2_delete_config(pl.conf)
	C.rs2_delete_pipeline(pl.p)
	C.rs2_delete_context(pl.ctx)
	return nil
}

func (pl *Pipeline) Start() error {
	var err *C.rs2_error
	prof := C.rs2_pipeline_start_with_config(pl.p, pl.conf, &err)
	if err != nil {
		return errorFrom(err)
	}

	pl.profile = prof
	return nil
}

func (pl *Pipeline) WaitColorFrames(colorFrame chan *gocv.Mat, timeout time.Duration) error {
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	var errc *C.rs2_error
	for {
		frames := C.rs2_pipeline_wait_for_frames(pl.p, C.uint(1000), &errc)
		if errc != nil {
			return errorFrom(errc)
		}

		count := C.rs2_embedded_frames_count(frames, &errc)
		if errc != nil {
			fmt.Println(errorFrom(errc))
			continue
		}

		for i := 0; i < int(count); i++ {
			var ret gocv.Mat
			var err error

			frame := C.rs2_extract_frame(frames, C.int(i), &errc)
			if frame == nil {
				continue
			}

			w := C.rs2_get_frame_width(frame, &errc)
			h := C.rs2_get_frame_height(frame, &errc)

			t := gocv.MatTypeCV8UC3
			var b []byte
			var sz C.int

			rgb_frame_data := C.rs2_get_frame_data(frame, &errc)

			if C.rs2_is_frame_extendable_to(frame, C.RS2_EXTENSION_DEPTH_FRAME, &errc) == 0 {
				t = gocv.MatTypeCV8UC3
				sz = w * h * 3
			} else {
				t = gocv.MatTypeCV16UC1
				sz = w * h * 2
			}

			b = C.GoBytes(unsafe.Pointer(rgb_frame_data), C.int(sz))
			ret, err = gocv.NewMatFromBytes(int(h), int(w), t, b)
			if err != nil {
				fmt.Println(err)
			} else {
				colorFrame <- &ret
			}

			C.rs2_release_frame(frame)

		}

		C.rs2_release_frame(frames)
	}
}
