//go:build windows

package windows

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/iaman221b/escpos-printer/device"
)

// Win32 print spooler bindings.
//
// Separated from backend.go because this is not backend behaviour: the finder
// depends on ListQueues and QueueOnline just as much as the backend does. What
// lives here is the winspool.drv surface itself — the procedure handles, the
// structures they read and write, the status bits they report, and the two
// queries built directly on them.

var (
	winspool = windows.NewLazySystemDLL("winspool.drv")

	procOpenPrinterW     = winspool.NewProc("OpenPrinterW")
	procClosePrinter     = winspool.NewProc("ClosePrinter")
	procStartDocPrinterW = winspool.NewProc("StartDocPrinterW")
	procEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	procStartPagePrinter = winspool.NewProc("StartPagePrinter")
	procEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	procWritePrinter     = winspool.NewProc("WritePrinter")
	procGetPrinterW      = winspool.NewProc("GetPrinterW")
	procEnumPrintersW    = winspool.NewProc("EnumPrintersW")
)

// docInfo1 mirrors DOC_INFO_1W.
type docInfo1 struct {
	DocName    *uint16
	OutputFile *uint16
	Datatype   *uint16
}

// printerInfo2 mirrors PRINTER_INFO_2W. Only Status and Attributes are read,
// but the whole struct must be laid out correctly for those offsets to land.
// Go applies the same natural alignment as the C compiler here, so no explicit
// padding is needed.
type printerInfo2 struct {
	ServerName         *uint16
	PrinterName        *uint16
	ShareName          *uint16
	PortName           *uint16
	DriverName         *uint16
	Comment            *uint16
	Location           *uint16
	DevMode            uintptr
	SepFile            *uint16
	PrintProcessor     *uint16
	Datatype           *uint16
	Parameters         *uint16
	SecurityDescriptor uintptr
	Attributes         uint32
	Priority           uint32
	DefaultPriority    uint32
	StartTime          uint32
	UntilTime          uint32
	Status             uint32
	CJobs              uint32
	AveragePPM         uint32
}

// printerInfo1 mirrors PRINTER_INFO_1W, used only to enumerate queue names.
type printerInfo1 struct {
	Flags       uint32
	Description *uint16
	Name        *uint16
	Comment     *uint16
}

const (
	printerEnumLocal       = 0x00000002
	printerEnumConnections = 0x00000004

	statusPaused           = 0x00000001
	statusError            = 0x00000002
	statusPaperJam         = 0x00000008
	statusPaperOut         = 0x00000010
	statusPaperProblem     = 0x00000040
	statusOffline          = 0x00000080
	statusNotAvailable     = 0x00001000
	statusUserIntervention = 0x00100000
	statusDoorOpen         = 0x00400000

	// printerAttributeWorkOffline is PRINTER_ATTRIBUTE_WORK_OFFLINE, from the
	// queue's Attributes word rather than its Status word. Windows sets it when
	// a USB printer is removed, and when a user chooses "Use Printer Offline".
	// See classifyQueueState for why Status alone is not enough.
	printerAttributeWorkOffline = 0x00000400
)

const coverOpenReason = "reporting its cover is open"

// classifyQueueState turns a print queue's raw status and attribute words into
// the Online/Paper fields, plus a reason for anything not healthy.
//
// Split out as a pure function so the mapping can be tested against real
// captured values without a printer or a Win32 call.
//
// The attribute check is the important one, and its absence is a classic bug:
// reading only Status reports an unplugged USB printer as connected. The
// spooler does not poll a USB device, so when the printer is removed its
// PRINTER_STATUS_* bits stay 0 — "nothing wrong has been reported" — which is
// not the same as "the printer is there". Windows records the removal in
// Attributes instead, as PRINTER_ATTRIBUTE_WORK_OFFLINE.
//
// Captured from an EPSON TM-T82X-II on USB001 with the cable unplugged:
//
//	Status     = 0      (no error bits at all)
//	Attributes = 1092   = 0x400 WORK_OFFLINE | 0x40 LOCAL | 0x04 DEFAULT
//
// The same flag is set when a user ticks "Use Printer Offline" in Windows.
// Reporting that as offline is correct too — nothing will print either way.
func classifyQueueState(status, attributes uint32) (online bool, paper device.PaperStatus, reason string) {
	paper = device.PaperOK
	switch {
	case status&(statusPaperOut|statusPaperJam) != 0:
		paper = device.PaperOut
	case status&(statusPaperProblem|statusUserIntervention) != 0:
		paper = device.PaperLow
	}

	// Checked first: it is the condition that catches an unplugged printer, and
	// the one the status bits are silent about.
	if attributes&printerAttributeWorkOffline != 0 {
		return false, paper, "offline — Windows has marked the queue offline, which usually means the printer is unplugged or powered off"
	}

	switch {
	case status&statusOffline != 0:
		return false, paper, "reported offline by the spooler"
	case status&statusNotAvailable != 0:
		return false, paper, "not available"
	case status&statusDoorOpen != 0:
		return false, paper, coverOpenReason
	case status&statusPaused != 0:
		return false, paper, "paused"
	case status&statusError != 0:
		return false, paper, "in an error state"
	}

	return true, paper, ""
}

// QueueOnline probes whether a Windows print queue is connected and healthy,
// without printing. Used by the Finder to skip disconnected printers during
// discovery — a settings screen should show only what is actually reachable,
// not every queue the spooler remembers.
//
// Returns (online, reason). When online is false, reason says why.
func QueueOnline(queueName string) (bool, string) {
	namePtr, err := windows.UTF16PtrFromString(queueName)
	if err != nil {
		return false, fmt.Sprintf("invalid queue name: %v", err)
	}

	var handle windows.Handle
	r1, _, lastErr := procOpenPrinterW.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(&handle)),
		0,
	)
	if r1 == 0 {
		return false, fmt.Sprintf("could not open queue: %v", lastErr)
	}
	defer procClosePrinter.Call(uintptr(handle))

	var needed uint32
	procGetPrinterW.Call(uintptr(handle), 2, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return false, "could not size printer info buffer"
	}

	buf := make([]byte, needed)
	r1, _, lastErr = procGetPrinterW.Call(
		uintptr(handle),
		2,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(needed),
		uintptr(unsafe.Pointer(&needed)),
	)
	if r1 == 0 {
		return false, fmt.Sprintf("could not read printer status: %v", lastErr)
	}

	info := (*printerInfo2)(unsafe.Pointer(&buf[0]))
	online, _, reason := classifyQueueState(info.Status, info.Attributes)
	return online, reason
}

// ListQueues returns the installed local print queue names. Also used to make a
// misconfigured queue name immediately obvious in an error instead of surfacing
// as an opaque "printer disconnected".
func ListQueues() []string {
	flags := uintptr(printerEnumLocal | printerEnumConnections)

	var needed, returned uint32
	r1, _, lastErr := procEnumPrintersW.Call(
		flags, 0, 1, 0, 0,
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	// The sizing call is expected to fail with ERROR_INSUFFICIENT_BUFFER.
	if r1 == 0 && needed == 0 {
		return []string{fmt.Sprintf("<enumeration failed: %v>", lastErr)}
	}
	if needed == 0 {
		return nil
	}

	buf := make([]byte, needed)
	r1, _, lastErr = procEnumPrintersW.Call(
		flags, 0, 1,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(needed),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if r1 == 0 {
		return []string{fmt.Sprintf("<enumeration failed: %v>", lastErr)}
	}

	names := make([]string, 0, returned)
	for i := uint32(0); i < returned; i++ {
		entry := (*printerInfo1)(unsafe.Pointer(&buf[uintptr(i)*unsafe.Sizeof(printerInfo1{})]))
		if entry.Name != nil {
			names = append(names, windows.UTF16PtrToString(entry.Name))
		}
	}
	return names
}
