#!/usr/bin/env swift
// Renders the 🎮 emoji into a macOS iconset, then call:
//   iconutil -c icns VRAMGuard.iconset -o pkg/VRAMGuard.icns
import Cocoa

NSApplication.shared.setActivationPolicy(.prohibited)

let emoji    = "🎮"
let iconset  = "VRAMGuard.iconset"

try? FileManager.default.createDirectory(atPath: iconset, withIntermediateDirectories: true)

func render(_ emoji: String, pixels: Int) -> Data? {
    guard let rep = NSBitmapImageRep(
        bitmapDataPlanes: nil,
        pixelsWide: pixels, pixelsHigh: pixels,
        bitsPerSample: 8, samplesPerPixel: 4,
        hasAlpha: true, isPlanar: false,
        colorSpaceName: .deviceRGB,
        bytesPerRow: 0, bitsPerPixel: 0
    ) else { return nil }

    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)

    NSColor.clear.setFill()
    NSRect(x: 0, y: 0, width: pixels, height: pixels).fill()

    let font  = NSFont.systemFont(ofSize: CGFloat(pixels) * 0.82)
    let attrs: [NSAttributedString.Key: Any] = [.font: font]
    let str   = NSAttributedString(string: emoji, attributes: attrs)
    let sz    = str.size()
    str.draw(at: NSPoint(
        x: (CGFloat(pixels) - sz.width)  / 2,
        y: (CGFloat(pixels) - sz.height) / 2
    ))

    NSGraphicsContext.restoreGraphicsState()
    return rep.representation(using: .png, properties: [:])
}

// Standard iconset spec: (filename, pixel-size)
let specs: [(String, Int)] = [
    ("icon_16x16",       16),
    ("icon_16x16@2x",    32),
    ("icon_32x32",       32),
    ("icon_32x32@2x",    64),
    ("icon_128x128",    128),
    ("icon_128x128@2x", 256),
    ("icon_256x256",    256),
    ("icon_256x256@2x", 512),
    ("icon_512x512",    512),
    ("icon_512x512@2x",1024),
]

for (name, px) in specs {
    guard let data = render(emoji, pixels: px) else {
        fputs("  FAIL \(name)\n", stderr); continue
    }
    let path = "\(iconset)/\(name).png"
    try? data.write(to: URL(fileURLWithPath: path))
    print("  \(path)")
}
