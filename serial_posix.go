// +build !windows

package serial

// #include <termios.h>
// #include <unistd.h>
import "C"

// TODO: Maybe change to using syscall package + ioctl instead of cgo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

type serialPort struct {
	f *os.File
}

func openPort(name string, baud int, spec []byte, flow []bool) (rwc io.ReadWriteCloser, err error) {
	port := new(serialPort)

	f, err := os.OpenFile(name, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0666)
	if err != nil {
		return
	}

	fd := C.int(f.Fd())
	if C.isatty(fd) != 1 {
		f.Close()
		return nil, errors.New("File is not a tty")
	}

	var st C.struct_termios
	_, err = C.tcgetattr(fd, &st)
	if err != nil {
		f.Close()
		return nil, err
	}
	var speed C.speed_t
	switch baud {
	case 115200:
		speed = C.B115200
	case 57600:
		speed = C.B57600
	case 38400:
		speed = C.B38400
	case 19200:
		speed = C.B19200
	case 9600:
		speed = C.B9600
	default:
		f.Close()
		return nil, fmt.Errorf("Unknown baud rate %v", baud)
	}

	_, err = C.cfsetispeed(&st, speed)
	if err != nil {
		f.Close()
		return nil, err
	}
	_, err = C.cfsetospeed(&st, speed)
	if err != nil {
		f.Close()
		return nil, err
	}

	// Select local mode
	st.c_cflag |= (C.CLOCAL | C.CREAD)

	// Select raw mode
	st.c_lflag &= ^C.tcflag_t(C.ICANON | C.ECHO | C.ECHOE | C.ISIG)
	st.c_oflag &= ^C.tcflag_t(C.OPOST)

	// Flow control
	if flow[RTS_FLAG] {
		st.c_cflag |= C.tcflag_t(C.CRTSCTS)
	}
	if flow[XON_FLAG] {
		st.c_cflag |= C.tcflag_t(C.IXON | C.IXOFF | C.IXANY)
	}

	// Defaults to 8N1 if nothing valid is given
	byteSize := spec[0]
	parity := spec[1]
	stopBits := spec[2]

	switch byteSize {
	case byte(5):
		st.c_cflag |= C.tcflag_t(C.CS5)
		break
	case byte(6):
		st.c_cflag |= C.tcflag_t(C.CS6)
		break
	case byte(7):
		st.c_cflag |= C.tcflag_t(C.CS7)
		break
	case byte(8):
	default:
		st.c_cflag |= C.tcflag_t(C.CS8)
		break
	}
	switch parity {
	case PARITY_EVEN:
		st.c_cflag |= C.tcflag_t(C.PARENB)
		st.c_cflag &= ^C.tcflag_t(C.PARODD)
		break
	case PARITY_ODD:
		st.c_cflag |= C.tcflag_t(C.PARENB)
		st.c_cflag |= C.tcflag_t(C.PARODD)
		break
	case PARITY_NONE:
	default:
		st.c_cflag &= ^C.tcflag_t(C.PARENB)
		break
	}
	switch stopBits {
	case byte(2):
		st.c_cflag |= C.tcflag_t(C.CSTOPB)
		break
	case byte(1):
	default:
		st.c_cflag &= ^C.tcflag_t(C.CSTOPB)
		break
	}
	st.c_cflag &= ^C.tcflag_t(C.CSIZE)

	_, err = C.tcsetattr(fd, C.TCSANOW, &st)
	if err != nil {
		f.Close()
		return nil, err
	}

	//fmt.Println("Tweaking", name)
	r1, _, e := syscall.Syscall(syscall.SYS_FCNTL,
		uintptr(f.Fd()),
		uintptr(syscall.F_SETFL),
		uintptr(0))
	if e != 0 || r1 != 0 {
		s := fmt.Sprint("Clearing NONBLOCK syscall error:", e, r1)
		f.Close()
		return nil, errors.New(s)
	}

	/*
				r1, _, e = syscall.Syscall(syscall.SYS_IOCTL,
			                uintptr(f.Fd()),
			                uintptr(0x80045402), // IOSSIOSPEED
			                uintptr(unsafe.Pointer(&baud)));
			        if e != 0 || r1 != 0 {
			                s := fmt.Sprint("Baudrate syscall error:", e, r1)
					f.Close()
		                        return nil, os.NewError(s)
				}
	*/

	port.f = f

	return port, nil
}

func (p *serialPort) SetTimeouts(msec uint32) {
}

func (p *serialPort) Read(buf []byte) (int, error) {
	return p.f.Read(buf)
}

func (p *serialPort) Write(buf []byte) (int, error) {
	return p.f.Write(buf)
}

func (p *serialPort) Close() error {
	return p.f.Close()
}
