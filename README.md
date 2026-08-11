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

Describe the receipt as a **document** — a slice of elements — and render it for a **profile**.
A receipt style is then just a function from your data to a `Document`, so adding a kitchen ticket
or a gift receipt is adding a function, not extending this library.

```go
func StandardReceipt(order Order) escpos.Document {
    doc := escpos.Document{
        escpos.Text{Value: order.Store, Bold: true, Align: escpos.AlignCenter},
        escpos.Text{Value: "Order #" + order.Number, Align: escpos.AlignCenter},
        escpos.Rule{},
    }
    for _, item := range order.Items {
        doc = append(doc, escpos.Row{Cells: []escpos.Cell{
            {Text: item.Name},                                        // flexible
            {Text: item.Qty,   Width: 4, Align: escpos.AlignRight},
            {Text: item.Total, Width: 7, Align: escpos.AlignRight},
        }})
    }
    return append(doc,
        escpos.Rule{},
        escpos.LeftRight("TOTAL", order.Total),
        escpos.QR{Data: "https://example.com/orders/" + order.Number, Align: escpos.AlignCenter},
        escpos.Cut{},
    )
}

data, err := escpos.Render(StandardReceipt(order), escpos.Profile58mm)
p.Print(ctx, data)
```

**`Row` is what makes a style paper-size independent.** Cells with `Width: 0` share whatever the
fixed columns leave, so the same document lays out correctly at 32 and 48 characters without the
style knowing which roll it is printing on.

**Layouts are testable as text.** The same document renders to something a golden file can diff:

```go
fmt.Println(escpos.RenderText(StandardReceipt(order), escpos.Profile58mm))
```
```
        **THE CORNER SHOP**
          Order #ORD-1043
--------------------------------
Flat white               x2  7.00
--------------------------------
TOTAL                       7.00
   [QR: https://example.com/...]
================================
             [CUT]
```

`Render` always returns bytes, even when an element fails — an invalid barcode payload costs you
the barcode, not the receipt. The error tells you what was dropped; log it rather than failing a
sale over it.

### Profiles

`Profile58mm` (32 columns, 384 dots) and `Profile80mm` (48 columns, 576 dots) are provided; build
your own for anything else. The profile also carries `CutFeed` and a `Capabilities` set — an
element the printer cannot draw is skipped and reported, never sent as a command it would render
as garbage.

### QR codes and barcodes

Both are drawn by the printer's **firmware** — the host sends the data and the printer encodes the
symbol. There is no client-side encoding, and no image to dither.

```go
escpos.QR{Data: "https://example.com"}                       // Model 2, module 6, correction M
escpos.Barcode{Data: "ORD-1043"}                             // Code128, HRI below
escpos.Barcode{Data: "1234567890128", Format: escpos.EAN13}
```

Code128 payloads are given a `{B` code-set prefix automatically — without one, Epson firmware
prints nothing at all, which is the most common way hand-rolled Code128 fails. Data a symbology
cannot represent is rejected with an error rather than printed as a symbol that scans to the wrong
value.

### Cutting

`Cut{}` feeds the paper clear of the cutter before cutting, using the profile's `CutFeed`. This is
not cosmetic: the print head sits ~15 mm above the blade, so cutting without feeding first slices
through blank leader and leaves the printed text stranded inside the printer — the slip that falls
out is blank. The API does it for you so you cannot get it wrong.

### The low-level Builder

`escpos.Builder` is still there as the escape hatch — chainable, imperative, and what `Render` uses
underneath, so there is one place in the library that emits bytes. Reach for it when you need a
command the document model does not cover; `Raw([]byte)` takes anything.

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
escposprinter                    the public API
├── registry.go                  state, selection, accessors
├── registry_discovery.go        running the finders, deciding what is active
├── printer.go                   the Printer handle
├── options.go                   With… options
├── options_configured.go        WithConfiguredDevice
├── store.go   errors.go   aliases.go
├── platform_windows.go          //go:build windows   ┐ the entire
├── platform_unix.go             //go:build !windows  ┘ platform surface
│
├── device/                      the shared vocabulary        (stdlib only)
│   device.go  backend.go  finder.go  errors.go  recognize.go
│
├── escpos/                      bytes and layout             (stdlib only)
│   ├── builder.go               Builder + core control sequences
│   ├── builder_barcode.go       GS k    — linear symbologies
│   ├── builder_drawer.go        ESC p   — the drawer pulse
│   ├── builder_qrcode.go        GS ( k  — QR codes
│   ├── document.go              Element types, Row layout
│   ├── document_profile.go      Profile, Capabilities
│   └── document_render.go       Render (bytes), RenderText (readable)
│
└── transport/                   each folder: doc.go, backend.go, finder.go
    ├── windows/                 print spooler   (+ winspool.go — Win32 bindings)
    ├── cups/                    lp / lpstat
    ├── usb/                     device nodes
    └── network/                 TCP 9100        (all platforms)
```

Filenames carry the grouping: everything under `builder_` is a command family the `Builder`
emits, everything under `document_` is the declarative layer built on top of it.

Dependencies flow one way: `root → transport/* → device/`. Import `escpos` alone if all you need is
bytes, or a single `transport/*` package if you know exactly which printer you are talking to.

## Status

Pre-v1. The API may still change; pin a version.

## License

MIT
