# escpos-printer

A Go library for ESC/POS thermal receipt printers: **find them, select one, print to it, check its
status, and open the cash drawer wired into it** — on Windows, Linux, and macOS.

```go
reg := escposprinter.New(escposprinter.WithStore(myDB))

devices, _ := reg.Discover(ctx)              // spooler on Windows, CUPS + USB on Unix
reg.Select(ctx, "windows:EPSON TM-T82X-II")  // remembered across restarts

p, _ := reg.Active()
p.Print(ctx, receiptBytes)
st, _ := p.Status(ctx)                       // st.Online, st.Paper
p.OpenDrawer(ctx, escpos.DefaultPulse())
```

## Why this exists

Go has plenty of packages that turn text into ESC/POS bytes. None of them answer the questions a
real point-of-sale application actually has: *which printers does this machine have? which one did
the operator pick? is it still plugged in? is it out of paper?*

Those answers live in the operating system — the Windows print spooler, CUPS, USB device nodes — and
they are different on every platform. This library handles that part, and picks the right transport
**at compile time**, so a Linux binary contains no Windows syscalls at all.

## Install

```
go get github.com/iaman221b/escpos-printer
```

Requires Go 1.25+. The only dependency is `golang.org/x/sys`, and only on Windows.

## Transports

| Transport | Windows | Linux | macOS | Sees paper state |
|---|:---:|:---:|:---:|:---:|
| Print spooler (`RAW` datatype) | ✅ | — | — | **yes** |
| CUPS (`lp -o raw`) | — | ✅ | ✅ | no |
| USB device node (`/dev/usb/lp*`) | — | ✅ | serial only | no |
| TCP port 9100 | ✅ | ✅ | ✅ | no |

USB is deliberately unsupported on Windows: a USB thermal printer there already appears as an
ordinary print queue, and talking to the device directly would mean replacing its driver.

Only the Windows spooler exposes a real paper signal. Elsewhere `Paper` reports `ok` because there
is no signal to read, and a genuine paper-out surfaces when a job fails. The library reports what it
can observe and nothing more.

## Building receipts

The `escpos` package produces bytes and never opens a connection, so one rendered stream prints
identically through any transport on any platform.

```go
b := escpos.NewBuilder()
b.DrawerKick(escpos.DefaultPulse())          // opens with the receipt, same job
b.Init().
    Align(escpos.AlignCenter).
    Bold(true).Line("THE CORNER SHOP").Bold(false).
    Rule(escpos.Width58mm).
    Align(escpos.AlignLeft).
    Line("Flat white          x2    7.00").
    Cut()

p.Print(ctx, b.Bytes())
```

`Cut()` feeds the paper clear of the cutter before cutting. This is not cosmetic: the print head
sits ~15 mm above the blade, so cutting without feeding first slices through blank leader and
leaves the printed text stranded inside the printer — the slip that falls out is blank. The API
does it for you so you cannot get it wrong. Use `CutAfter(n)` if your printer's gap differs.

## Remembering the operator's choice

The library never opens a database. It defines a two-method interface and you implement it over
whatever storage you already have:

```go
type SelectionStore interface {
    LoadSelectedPrinterID(ctx context.Context) (string, error)
    SaveSelectedPrinterID(ctx context.Context, id string) error
}
```

Pass it with `WithStore`, then call `RestoreSelection` after the first `Discover`. A `MemoryStore`
is provided for tests. A nil store is valid — selections just don't outlive the process.

## How a printer gets selected

In order of precedence:

1. **A pin** (`WithPin`) — a terminal that names its printer gets that printer, every time.
2. **The remembered selection**, if that printer is still attached.
3. **The automatic pick** — but only when there is exactly one recognised receipt printer, or
   exactly one printer of any kind.

When a pinned printer is *missing*, printing is disabled rather than falling back to another
device. Receipts surfacing at the wrong counter is worse than receipts not surfacing at all,
because nobody notices it happening.

## Errors

Failures wrap sentinels, so you branch on the condition rather than on message text:

```go
switch {
case errors.Is(err, escposprinter.ErrPaperOut):     // reload the roll
case errors.Is(err, escposprinter.ErrCoverOpen):    // close the lid
case errors.Is(err, escposprinter.ErrDisconnected): // check the cable
case errors.Is(err, escposprinter.ErrNoPrinter):    // nothing attached at all
}
```

`ErrNoPrinter` is an answer, not a malfunction — a till with nothing plugged in still takes
payments, it just cannot produce paper.

## The cash drawer

A retail cash drawer is almost never a peripheral in its own right: it is an RJ-11 cable from the
drawer into the receipt printer's DK port. "Open the drawer" means sending five bytes to the
printer, which is why `OpenDrawer` lives on `Printer`.

The DK port has **no read-back**. A printer accepts the kick identically whether or not a drawer is
attached, so a success here means the pulse was delivered — never that the drawer opened. This
library will not claim otherwise.

## What this library does not do

- **Read environment variables.** Configuration is passed in through `Option`s; your application
  reads its own settings.
- **Own a database.** See `SelectionStore`.
- **Define what a receipt contains.** Store names, line items, tax, and totals are your business
  contract. `escpos.Builder` gives you primitives; the layout is yours.
- **Ship a mock printer that reports itself healthy.** "No printer" is the absence of a backend, not
  a backend that pretends. A fake that answers `Online` makes every diagnostics screen lie.

## Examples

```
go run ./examples/list      # enumerate printers
go run ./examples/print     # print a sample receipt
go run ./examples/drawer    # kick the drawer
```

## Package layout

```
escposprinter        Registry, Printer, Options, SelectionStore, errors
├── device/          Device, Backend, Finder, Status — the shared vocabulary (stdlib only)
├── escpos/          byte rendering and the drawer pulse (stdlib only)
└── transport/
    ├── windows/     print spooler          (//go:build windows)
    ├── cups/        lp / lpstat            (//go:build !windows)
    ├── usb/         device nodes           (//go:build !windows)
    └── network/     TCP 9100               (all platforms)
```

Dependencies flow one way: `root → transport/* → device/`. Import `escpos` alone if all you need is
bytes, or a single `transport/*` package if you know exactly which printer you are talking to.

## Status

Pre-v1. The API may still change; pin a version.

## License

MIT
