package librealsense2

/*
#cgo linux darwin LDFLAGS: -L/usr/local/lib/ -lrealsense2
#cgo CPPFLAGS: -I/usr/local/include
#include <librealsense2/rs.h>
#include <librealsense2/h/rs_pipeline.h>
#define STREAM          RS2_STREAM_DEPTH  // rs2_stream is a types of data provided by RealSense device           //
#define FORMAT          RS2_FORMAT_Z16    // rs2_format identifies how binary data is encoded within a frame      //
#define WIDTH           640               // Defines the number of columns for each frame                         //
#define HEIGHT          480               // Defines the number of lines for each frame                           //
#define FPS             30                // Defines the rate of frames per second                                //
#define STREAM_INDEX    0
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"

	"gocv.io/x/gocv"
)

var (
	ErrNotDevicesFound = errors.New("no realsense devices found")
)

type Pipeline struct {
	p       *C.rs2_pipeline
	ctx     *C.rs2_context
	conf    *C.rs2_config
	profile *C.rs2_pipeline_profile
}

type PipelineConfig struct {
	Serial string
}

func NewPipeline(cfg *PipelineConfig) (*Pipeline, error) {
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

	C.rs2_config_enable_stream(conf, C.STREAM, C.STREAM_INDEX, C.WIDTH, C.HEIGHT, C.FORMAT, C.FPS, &err)

	if err != nil {
		return nil, errorFrom(err)
	}

	C.rs2_delete_device_list(device_list)

	return &Pipeline{
		p:    p,
		ctx:  ctx,
		conf: conf,
	}, nil
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

func (pl *Pipeline) WaitColorFrames(colorFrame chan *gocv.Mat) {
	var err *C.rs2_error
	for {
		frames := C.rs2_pipeline_wait_for_frames(pl.p, C.RS2_DEFAULT_TIMEOUT, &err)
		if err != nil {
			fmt.Println(errorFrom(err))
			continue
		}

		count := C.rs2_embedded_frames_count(frames, &err)
		if err != nil {
			fmt.Println(errorFrom(err))
			continue
		}

		for i := 0; i < int(count); i++ {
			frame := C.rs2_extract_frame(frames, C.int(i), &err)
			if C.rs2_is_frame_extendable_to(frame, C.RS2_EXTENSION_DEPTH_FRAME, &err) == 0 {
				C.rs2_release_frame(frame)
				continue
			}

			//fmt.Println(size)

			rgb_frame_data := C.rs2_get_frame_data(frame, &err)
			b := C.GoBytes(unsafe.Pointer(rgb_frame_data), 640*480*2)

			ret, errg := gocv.NewMatFromBytes(480, 640, gocv.MatTypeCV16UC1, b)
			if errg != nil {
				fmt.Println(errg)
			} else {
				colorFrame <- &ret
			}

			C.rs2_release_frame(frame)

		}

		C.rs2_release_frame(frames)
	}
}
