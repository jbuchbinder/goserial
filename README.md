# GOSERIAL

A simple go package to allow you to read and write from the
serial port as a stream of bytes.

## DETAILS

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
timeouts in Mac/Linux, though patches are welcome.

You may Read() and Write() simulantiously on the same connection (from
different goroutines).

## USAGE

```go
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
```

## POSSIBLE FUTURE WORK

* better tests (loopback etc)
* timeout support for Linux/Mac
* more low-level setting and configuration for serial ports

## REFERENCES

* Win32 Serial Communication : http://msdn.microsoft.com/en-us/library/ms810467.aspx
* Windows DCB structure : http://msdn.microsoft.com/en-us/library/windows/desktop/aa363214(v=vs.85).aspx

