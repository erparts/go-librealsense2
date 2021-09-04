package librealsense2

/*
#cgo linux darwin LDFLAGS: -L/usr/local/lib/ -lrealsense2
#cgo CPPFLAGS: -I/usr/local/include
#include <librealsense2/rs.h>
*/
import "C"
import "fmt"

func errorFrom(err *C.rs2_error) error {
	defer C.rs2_free_error(err)
	return fmt.Errorf(C.GoString(C.rs2_get_error_message(err)))
}
