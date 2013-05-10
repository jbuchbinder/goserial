// +build windows

package serial

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

const (
	SERIAL_FLAGS_CLEAR = 0

	FLAG_DTRCONTROL = 0
	FLAG_RTSCONTROL = 1
	OFFSET_DTRCONTROL = 4
	OFFSET_RTSCONTROL = 4

	// Imported from winbase.h
	RTS_CONTROL_DISABLE     = 0 /* 0b00 */ << OFFSET_RTSCONTROL
	RTS_CONTROL_ENABLE      = 1 /* 0b01 */ << OFFSET_RTSCONTROL
	RTS_CONTROL_HANDSHAKE   = 2 /* 0b10 */ << OFFSET_RTSCONTROL
	RTS_CONTROL_TOGGLE      = 3 /* 0b11 */ << OFFSET_RTSCONTROL
	DTR_CONTROL_DISABLE     = 0 /* 0b00 */ << OFFSET_DTRCONTROL
	DTR_CONTROL_ENABLE      = 1 /* 0b01 */ << OFFSET_DTRCONTROL
	DTR_CONTROL_HANDSHAKE   = 2 /* 0b10 */ << OFFSET_DTRCONTROL
)

type serialPort struct {
	f  *os.File
	fd syscall.Handle
	rl sync.Mutex
	wl sync.Mutex
	st *structTimeouts
	h syscall.Handle
	ro *syscall.Overlapped
	wo *syscall.Overlapped
}

type structDCB struct {
	DCBlength, BaudRate                            uint32
	flags                                          [4]byte
	wReserved, XonLim, XoffLim                     uint16
	ByteSize, Parity, StopBits                     byte
	XonChar, XoffChar, ErrorChar, EofChar, EvtChar byte
	wReserved1                                     uint16
}

type structTimeouts struct {
	ReadIntervalTimeout         uint32
	ReadTotalTimeoutMultiplier  uint32
	ReadTotalTimeoutConstant    uint32
	WriteTotalTimeoutMultiplier uint32
	WriteTotalTimeoutConstant   uint32
}

func openPort(name string, baud int, spec []byte, flow []bool) (rwc io.ReadWriteCloser, err error) {
	if len(name) > 0 && name[0] != '\\' {
		name = "\\\\.\\" + name
	}

	h, err := syscall.CreateFile(syscall.StringToUTF16Ptr(name),
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OVERLAPPED,
		0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), name)
	defer func() {
		if err != nil {
			f.Close()
		}
	}()

	// TODO: Sanity checking for these comm parameters
	byteSize := spec[0]
	stopBits := spec[1]
	parity := spec[2]
	log.Printf("DEBUG: byteSize = %d, parity = %d, stopBits = %d", int(byteSize), int(stopBits), int(parity))

	if err = setCommState(h, baud, byteSize, stopBits, parity, flow); err != nil {
		log.Print("Failed to setCommState")
		return
	}
	if err = setupComm(h, 64, 64); err != nil {
		log.Print("Failed to setupComm")
		return
	}
	if err = setCommMask(h); err != nil {
		log.Print("Failed to setCommMask")
		return
	}

	ro, err := newOverlapped()
	if err != nil {
		log.Print("Failed to set ro with newOverlapped")
		return
	}
	wo, err := newOverlapped()
	if err != nil {
		log.Print("Failed to set wo with newOverlapped")
		return
	}
	port := new(serialPort)
	port.f = f
	port.fd = h
	port.ro = ro
	port.wo = wo
	var timeouts structTimeouts
	port.st = &timeouts
	port.SetTimeouts(100)

	return port, nil
}

func (p *serialPort) Close() error {
	return p.f.Close()
}

func (p *serialPort) SetTimeouts(msec uint32){
	timeouts := p.st
	timeouts.ReadIntervalTimeout = msec/10
	timeouts.ReadTotalTimeoutMultiplier = msec
	timeouts.ReadTotalTimeoutConstant = msec

	/* From http://msdn.microsoft.com/en-us/library/aa363190(v=VS.85).aspx

		 For blocking I/O see below:

		 Remarks:

		 If an application sets ReadIntervalTimeout and
		 ReadTotalTimeoutMultiplier to MAXDWORD and sets
		 ReadTotalTimeoutConstant to a value greater than zero and
		 less than MAXDWORD, one of the following occurs when the
		 ReadFile function is called:

		 If there are any bytes in the input buffer, ReadFile returns
		       immediately with the bytes in the buffer.

		 If there are no bytes in the input buffer, ReadFile waits
	               until a byte arrives and then returns immediately.

		 If no bytes arrive within the time specified by
		       ReadTotalTimeoutConstant, ReadFile times out.
	*/

    p.st = timeouts
    setCommTimeouts(p.h, timeouts)
}

func (p *serialPort) Write(buf []byte) (int, error) {
	p.wl.Lock()
	defer p.wl.Unlock()

	if err := resetEvent(p.wo.HEvent); err != nil {
		return 0, err
	}
	var n uint32
	err := syscall.WriteFile(p.fd, buf, &n, p.wo)
	if err != nil && err != syscall.ERROR_IO_PENDING {
		return int(n), err
	}
	return getOverlappedResult(p.fd, p.wo)
}

func (p *serialPort) Read(buf []byte) (int, error) {
	if p == nil || p.f == nil {
		return 0, fmt.Errorf("Invalid port on read %v %v", p, p.f)
	}

	p.rl.Lock()
	defer p.rl.Unlock()

	if err := resetEvent(p.ro.HEvent); err != nil {
		return 0, err
	}
	var done uint32
	err := syscall.ReadFile(p.fd, buf, &done, p.ro)
	if err != nil && err != syscall.ERROR_IO_PENDING {
		return int(done), err
	}
	return getOverlappedResult(p.fd, p.ro)
}

var (
	nSetCommState,
	nSetCommTimeouts,
	nSetCommMask,
	nSetupComm,
	nGetOverlappedResult,
	nCreateEvent,
	nResetEvent uintptr
)

func init() {
	k32, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		panic("LoadLibrary " + err.Error())
	}
	defer syscall.FreeLibrary(k32)

	nSetCommState = getProcAddr(k32, "SetCommState")
	nSetCommTimeouts = getProcAddr(k32, "SetCommTimeouts")
	nSetCommMask = getProcAddr(k32, "SetCommMask")
	nSetupComm = getProcAddr(k32, "SetupComm")
	nGetOverlappedResult = getProcAddr(k32, "GetOverlappedResult")
	nCreateEvent = getProcAddr(k32, "CreateEventW")
	nResetEvent = getProcAddr(k32, "ResetEvent")
}

func getProcAddr(lib syscall.Handle, name string) uintptr {
	addr, err := syscall.GetProcAddress(lib, name)
	if err != nil {
		panic(name + " " + err.Error())
	}
	return addr
}

func setCommState(h syscall.Handle, baud int, byteSize, stopBits, parity byte, flow []bool) error {
	var params structDCB
	params.DCBlength = uint32(unsafe.Sizeof(params))

	params.flags[0] = SERIAL_FLAGS_CLEAR
	params.flags[0] |= 1  // fBinary (0b01)

        if flow[DTR_FLAG] {
		params.flags[FLAG_DTRCONTROL] |= DTR_CONTROL_ENABLE // Assert DSR
        } else {
		params.flags[FLAG_DTRCONTROL] |= DTR_CONTROL_HANDSHAKE // Assert DSR
	}

        if flow[RTS_FLAG] {
		params.flags[FLAG_RTSCONTROL] |= RTS_CONTROL_ENABLE // Assert RTS/CTS
        } else {
		params.flags[FLAG_RTSCONTROL] |= RTS_CONTROL_HANDSHAKE // Assert RTS/CTS
	}

	params.BaudRate = uint32(baud)
	params.ByteSize = byteSize
	params.Parity = parity
	params.StopBits = stopBits

	r, _, err := syscall.Syscall(nSetCommState, 2, uintptr(h), uintptr(unsafe.Pointer(&params)), 0)
	if r == 0 {
		return err
	}
	return nil
}

func setCommTimeouts(h syscall.Handle, timeouts *structTimeouts) error {
	r, _, err := syscall.Syscall(nSetCommTimeouts, 2, uintptr(h), uintptr(unsafe.Pointer(timeouts)), 0)
	if r == 0 {
		return err
	}
	return nil
}

func setupComm(h syscall.Handle, in, out int) error {
	r, _, err := syscall.Syscall(nSetupComm, 3, uintptr(h), uintptr(in), uintptr(out))
	if r == 0 {
		return err
	}
	return nil
}

func setCommMask(h syscall.Handle) error {
	const EV_RXCHAR = 1 /* 0b0001 */
	r, _, err := syscall.Syscall(nSetCommMask, 2, uintptr(h), EV_RXCHAR, 0)
	if r == 0 {
		return err
	}
	return nil
}

func resetEvent(h syscall.Handle) error {
	r, _, err := syscall.Syscall(nResetEvent, 1, uintptr(h), 0, 0)
	if r == 0 {
		return err
	}
	return nil
}

func newOverlapped() (*syscall.Overlapped, error) {
	var overlapped syscall.Overlapped
	r, _, err := syscall.Syscall6(nCreateEvent, 4, 0, 1, 0, 0, 0, 0)
	if r == 0 {
		return nil, err
	}
	overlapped.HEvent = syscall.Handle(r)
	return &overlapped, nil
}

func getOverlappedResult(h syscall.Handle, overlapped *syscall.Overlapped) (int, error) {
	var n int
	r, _, err := syscall.Syscall6(nGetOverlappedResult, 4,
		uintptr(h),
		uintptr(unsafe.Pointer(overlapped)),
		uintptr(unsafe.Pointer(&n)), 1, 0, 0)
	if r == 0 {
		return n, err
	}

	return n, nil
}
