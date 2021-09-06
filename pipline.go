package librealsense2

/*
#cgo linux darwin LDFLAGS: -L/usr/local/lib/ -lrealsense2
#cgo CPPFLAGS: -I/usr/local/include
#include <librealsense2/rs.h>
#include <librealsense2/h/rs_pipeline.h>
#include <stdio.h>
#define STREAM          RS2_STREAM_COLOR  // rs2_stream is a types of data provided by RealSense device           //
#define FORMAT          RS2_FORMAT_RGB8   // rs2_format identifies how binary data is encoded within a frame      //
#define WIDTH           640               // Defines the number of columns for each frame                         //
#define HEIGHT          480               // Defines the number of lines for each frame                           //
#define FPS             30                // Defines the rate of frames per second                                //
#define STREAM_INDEX    0
	void print_device_info(rs2_device* dev){
    rs2_error* e = 0;
    printf("\nUsing device 0, an %s\n", rs2_get_device_info(dev, RS2_CAMERA_INFO_NAME, &e));
    printf("    Serial number: %s\n", rs2_get_device_info(dev, RS2_CAMERA_INFO_SERIAL_NUMBER, &e));
    printf("    Firmware version: %s\n\n", rs2_get_device_info(dev, RS2_CAMERA_INFO_FIRMWARE_VERSION, &e));
}
*/
import "C"
import (
	"fmt"
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
	}
	device_list := C.rs2_query_devices(ctx, &err)
	if err != nil {
		panic(errorFrom(err))
	}

	dev_count := C.rs2_get_device_count(device_list, &err)
	if err != nil {
		panic(errorFrom(err))
	}
	fmt.Printf("There are %d connected RealSense devices.\n", int(dev_count))
	if C.int(dev_count) == 0 {
		panic("cannot found device")
	}
	for i := 0; i < int(dev_count); i++ {
		dev := C.rs2_create_device(device_list, C.int(i), &err)
		if err != nil {
			fmt.Println(errorFrom(err))
			continue
		}
		C.print_device_info(dev)
		C.rs2_delete_device(dev)
	}
	p := C.rs2_create_pipeline(ctx, &err)
	if err != nil {
		panic(errorFrom(err))
	}
	conf := C.rs2_create_config(&err)
	if err != nil {
		panic(errorFrom(err))
	}
	C.rs2_config_enable_stream(conf, C.STREAM, C.STREAM_INDEX, C.WIDTH, C.HEIGHT, C.FORMAT, C.FPS, &err)

	C.rs2_delete_device_list(device_list)
	return &Pipeline{
		p:    p,
		ctx:  ctx,
		conf: conf,
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
			rgb_frame_data := C.rs2_get_frame_data(frame, &err)
			b := C.GoBytes(unsafe.Pointer(rgb_frame_data), 640*480*3)
			ret, errg := gocv.NewMatFromBytes(640, 480, gocv.MatTypeCV8SC3, b)
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
