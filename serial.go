/*
	Goserial is a simple go package to allow you to read and write from
	the serial port as a stream of bytes.

	It aims to have the same API on all platforms, including windows.  As
	an added bonus, the windows package does not use cgo, so you can cross
	compile for windows from another platform.  Unfortunately goinstall
	does not currently let you cross compile so you will have to do it
	manually:

	 GOOS=windows make clean install

	Currently there is very little in the way of configurability.  You can
	set the baud rate.  Then you can Read(), Write(), or Close() the
	connection.  Read() will block until at least one byte is returned.
	Write is the same.  There is currently no exposed way to set the
	timeouts, though patches are welcome.

	Currently all ports are opened with 8 data bits, 1 stop bit, no
	parity, no hardware flow control, and no software flow control.  This
	works fine for many real devices and many faux serial devices
	including usb-to-serial converters and bluetooth serial ports.

	You may Read() and Write() simulantiously on the same connection (from
	different goroutines).

	Example usage:

	  package main

	  import (
		serial "github.com/jbuchbinder/goserial"
		"log"
	  )

	  func main() {
		c := &serial.Config{
			Name: "COM45",
			Baud: 115200,
			Size: 8,
			Parity: serial.PARITY_NONE,
			StopBits: 1,
			RTSFlowControl: false,
			DTRFlowControl: false,
			XONFlowControl: false,
			Timeout: 0
		}
		s, err := serial.OpenPort(c)
        if err != nil {
                log.Fatal(err)
        }

        n, err := s.Write([]byte("test"))
        if err != nil {
                log.Fatal(err)
        }

        buf := make([]byte, 128)
        n, err = s.Read(buf)
        if err != nil {
                log.Fatal(err)
        }
        log.Print("%q", buf[:n])
  }
*/
package serial

import "io"

const (
	RTS_FLAG     = 0
	DTR_FLAG     = 1
	XON_FLAG     = 2
	PARITY_NONE  = byte(0)
	PARITY_EVEN  = byte(2)
	PARITY_ODD   = byte(1)
	PARITY_MARK  = byte(3)
	PARITY_SPACE = byte(4)
)

// Config contains the information needed to open a serial port.
//
// Currently few options are implemented, but more may be added in the
// future (patches welcome), so it is recommended that you create a
// new config addressing the fields by name rather than by order.
//
// For example:
//
//    c0 := &serial.Config{Name: "COM45", Baud: 115200}
// or
//    c1 := new(serial.Config)
//    c1.Name = "/dev/tty.usbserial"
//    c1.Baud = 115200
//
type Config struct {
	Name string
	Baud int

	Size     int // 0 get translated to 8
	Parity   byte
	StopBits int

	RTSFlowControl bool
	DTRFlowControl bool
	XONFlowControl bool

	// CRLFTranslate bool
	Timeout int
}

// OpenPort opens a serial port with the specified configuration
func OpenPort(c *Config) (io.ReadWriteCloser, error) {
	spec := make([]byte, 3)
	spec[0] = byte(c.Size)
	if spec[0] == byte(0) {
		spec[0] = byte(8)
	}
	spec[1] = c.Parity
	spec[2] = byte(c.StopBits)

	// Determine "flow" flags based on config
	flow := []bool{c.RTSFlowControl, c.DTRFlowControl, c.XONFlowControl}

	return openPort(c.Name, c.Baud, spec, flow)
}

// func Flush()

// func SendBreak()

// func RegisterBreakHandler(func())
