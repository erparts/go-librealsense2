package librealsense2

/*
#cgo linux darwin LDFLAGS: -L/usr/local/lib/ -lrealsense2
#cgo CPPFLAGS: -I/usr/local/include
#include <librealsense2/rs.h>
#include <librealsense2/h/rs_pipeline.h>
*/
import "C"
import (
	"unsafe"

	"gocv.io/x/gocv"
)

type Pipeline struct {
	p       *C.rs2_pipeline
	ctx     *C.rs2_context
	conf    *C.rs2_config
	profile *C.rs2_pipeline_profile
}

func NewPipeline() *Pipeline {
	var err *C.rs2_error
	ctx := C.rs2_create_context(C.RS2_API_VERSION, &err)
	if err != nil {
		panic(errorFrom(err))
		return nil
	}
	p := C.rs2_create_pipeline(ctx, &err)
	if err != nil {
		panic(errorFrom(err))
		return nil
	}
	conf := C.rs2_create_config(&err)
	if err != nil {
		panic(errorFrom(err))
		return nil
	}
	C.rs2_config_enable_stream(conf, C.RS2_STREAM_COLOR, C.int(0), C.int(640), C.int(480), C.RS2_FORMAT_RGB8, C.int(30), &err)
	prof := C.rs2_pipeline_start_with_config(p, conf, &err)
	if err != nil {
		panic(errorFrom(err))
		return nil
	}
	return &Pipeline{
		p:       p,
		ctx:     ctx,
		conf:    conf,
		profile: prof,
	}
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
	return nil
}

func (pl *Pipeline) Start() error {
	var err *C.rs2_error
	C.rs2_pipeline_start_with_config(pl.p, pl.conf, &err)
	if err != nil {
		return errorFrom(err)
	}
	return nil
}

func (pl *Pipeline) WaitColorFrames(colorFrame chan *gocv.Mat) {
	var err *C.rs2_error
	for {
		frames := C.rs2_pipeline_wait_for_frames(pl.p, C.RS2_DEFAULT_TIMEOUT, &err)
		if err != nil {
			continue
		}
		count := C.rs2_embedded_frames_count(frames, &err)
		if err != nil {
			continue
		}
		for i := 0; i < int(count); i++ {
			frame := C.rs2_extract_frame(frames, C.int(i), &err)
			rgb_frame_data := C.rs2_get_frame_data(frame, &err)
			b := C.GoBytes(unsafe.Pointer(rgb_frame_data), 640*480*3)
			ret, errg := gocv.NewMatFromBytes(640, 480, gocv.MatTypeCV8SC3, b)
			if errg != nil {
				continue
			}
			colorFrame <- &ret
		}
	}

}
