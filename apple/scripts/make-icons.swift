// Draws the tvOS app icon layers and top shelf images from the web icon's
// design (web/public/icon.svg): a tartan play triangle on dark brown.
// Run from apple/: swift scripts/make-icons.swift
import AppKit
import CoreGraphics
import ImageIO
import UniformTypeIdentifiers

let out = "PLAID/Resources/Assets.xcassets/App Icon & Top Shelf Image.brandassets"

func rgb(_ hex: UInt32, _ a: CGFloat = 1, lift: CGFloat = 1) -> CGColor {
    let r = min(1, CGFloat((hex >> 16) & 0xff) / 255 * lift)
    let g = min(1, CGFloat((hex >> 8) & 0xff) / 255 * lift)
    let b = min(1, CGFloat(hex & 0xff) / 255 * lift)
    return CGColor(srgbRed: r, green: g, blue: b, alpha: a)
}

func context(_ w: Int, _ h: Int) -> CGContext {
    let c = CGContext(data: nil, width: w, height: h, bitsPerComponent: 8, bytesPerRow: 0,
                      space: CGColorSpace(name: CGColorSpace.sRGB)!,
                      bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
    // SVG coordinates: origin top-left.
    c.translateBy(x: 0, y: CGFloat(h))
    c.scaleBy(x: 1, y: -1)
    return c
}

func save(_ c: CGContext, _ path: String) {
    let url = URL(fileURLWithPath: path)
    try? FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
    let dest = CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil)!
    CGImageDestinationAddImage(dest, c.makeImage()!, nil)
    CGImageDestinationFinalize(dest)
}

/// The dark brown backdrop with the amber glow.
func drawBack(_ c: CGContext, _ w: CGFloat, _ h: CGFloat, glowAt: CGPoint) {
    let grad = CGGradient(colorsSpace: nil, colors: [rgb(0x2b2620), rgb(0x110f0c)] as CFArray, locations: [0, 1])!
    c.drawLinearGradient(grad, start: .zero, end: CGPoint(x: 0, y: h), options: [])
    let glow = CGGradient(colorsSpace: nil, colors: [rgb(0xffbc60, 0.30), rgb(0xffbc60, 0)] as CFArray, locations: [0, 1])!
    c.drawRadialGradient(glow, startCenter: glowAt, startRadius: 0, endCenter: glowAt, endRadius: max(w, h) * 0.55, options: [])
}

/// One 120-unit sett of the tartan, as in the web icon's pattern.
func drawSett(_ c: CGContext, at o: CGPoint, unit u: CGFloat) {
    let lift: CGFloat = 1.75
    func rect(_ x: CGFloat, _ y: CGFloat, _ w: CGFloat, _ h: CGFloat, _ col: UInt32, _ a: CGFloat, l: CGFloat = lift) {
        c.setFillColor(rgb(col, a, lift: l))
        c.fill(CGRect(x: o.x + x * u, y: o.y + y * u, width: w * u, height: h * u))
    }
    rect(0, 0, 120, 120, 0x1d2a3a, 1)
    rect(0, 0, 60, 120, 0x1f3a30, 0.85)
    rect(0, 0, 120, 60, 0x1f3a30, 0.55)
    rect(22, 0, 16, 120, 0x0d1218, 0.7)
    rect(0, 22, 120, 16, 0x0d1218, 0.6)
    rect(88, 0, 6, 120, 0x0d1218, 0.6)
    rect(0, 88, 120, 6, 0x0d1218, 0.5)
    rect(58, 0, 4, 120, 0xffbc60, 1, l: 1)
    rect(0, 58, 120, 4, 0xffbc60, 0.9, l: 1)
    rect(104, 0, 2, 120, 0xf4ecdc, 0.35, l: 1)
    rect(0, 104, 120, 2, 0xf4ecdc, 0.3, l: 1)
}

/// The play triangle of the web icon (1024 viewBox), scaled into `box`.
func playPath(in box: CGRect) -> CGPath {
    let s = box.height / 1024
    var t = CGAffineTransform(translationX: box.midX - 590 * s, y: box.minY).scaledBy(x: s, y: s)
    let p = CGMutablePath()
    p.move(to: CGPoint(x: 372, y: 262))
    p.addLine(to: CGPoint(x: 372, y: 762))
    p.addQuadCurve(to: CGPoint(x: 410, y: 784), control: CGPoint(x: 372, y: 806))
    p.addLine(to: CGPoint(x: 790, y: 548))
    p.addQuadCurve(to: CGPoint(x: 790, y: 500), control: CGPoint(x: 826, y: 524))
    p.addLine(to: CGPoint(x: 410, y: 240))
    p.addQuadCurve(to: CGPoint(x: 372, y: 262), control: CGPoint(x: 372, y: 218))
    p.closeSubpath()
    return p.copy(using: &t)!
}

func drawPlay(_ c: CGContext, in box: CGRect) {
    let path = playPath(in: box)
    let s = box.height / 1024
    // Soft drop shadow.
    c.saveGState()
    c.setShadow(offset: CGSize(width: 0, height: -22 * s), blur: 60 * s, color: rgb(0x000000, 0.55))
    c.addPath(path); c.setFillColor(rgb(0x1d2a3a)); c.fillPath()
    c.restoreGState()
    // Tartan fill.
    c.saveGState()
    c.addPath(path); c.clip()
    let u = s * 1.5
    let r = path.boundingBox
    var y = r.minY
    while y < r.maxY { var x = r.minX; while x < r.maxX { drawSett(c, at: CGPoint(x: x, y: y), unit: u); x += 120 * u }; y += 120 * u }
    // Twill: fine diagonal lines.
    c.setStrokeColor(rgb(0x000000, 0.14)); c.setLineWidth(3 * s)
    var d = r.minX - r.height
    while d < r.maxX { c.move(to: CGPoint(x: d, y: r.maxY)); c.addLine(to: CGPoint(x: d + r.height, y: r.minY)); d += 8 * s * 1.4 }
    c.strokePath()
    // Fold: light at the top, shade at the bottom.
    let fold = CGGradient(colorsSpace: nil, colors: [rgb(0xffffff, 0.16), rgb(0xffffff, 0), rgb(0x000000, 0.25)] as CFArray, locations: [0, 0.12, 1])!
    c.drawLinearGradient(fold, start: CGPoint(x: 0, y: r.minY), end: CGPoint(x: 0, y: r.maxY), options: [])
    c.restoreGState()
    // Cream edge.
    c.addPath(path); c.setStrokeColor(rgb(0xf4ecdc, 0.55)); c.setLineWidth(10 * s); c.strokePath()
}

func drawWordmark(_ c: CGContext, _ text: String, at p: CGPoint, size: CGFloat) {
    CTFontManagerRegisterFontsForURL(URL(fileURLWithPath: "PLAID/Resources/Fonts/InstrumentSans-Variable-latin.ttf") as CFURL, .process, nil)
    let wght = 0x77676874 // 'wght'
    let desc = CTFontDescriptorCreateWithAttributes([
        kCTFontNameAttribute: "InstrumentSans-Regular",
        kCTFontVariationAttribute: [wght: 700],
    ] as CFDictionary)
    let font = CTFontCreateWithFontDescriptor(desc, size, nil) as NSFont
    let attr = NSAttributedString(string: text, attributes: [
        .font: font, .foregroundColor: NSColor(srgbRed: 0.957, green: 0.941, blue: 0.914, alpha: 1), .kern: -size * 0.035,
    ])
    let line = CTLineCreateWithAttributedString(attr)
    c.saveGState()
    c.textMatrix = .identity
    c.translateBy(x: p.x, y: p.y)
    c.scaleBy(x: 1, y: -1)
    CTLineDraw(line, c)
    c.restoreGState()
}

// MARK: App icon: back and front layers, 400×240 @1x/@2x and 1280×768.

struct Size { let name: String; let w: Int; let h: Int; let scale: Int }
let icons: [(folder: String, sizes: [Size])] = [
    ("App Icon.imagestack", [Size(name: "icon", w: 400, h: 240, scale: 1), Size(name: "icon@2x", w: 800, h: 480, scale: 2)]),
    ("App Icon - App Store.imagestack", [Size(name: "icon", w: 1280, h: 768, scale: 1)]),
]

for icon in icons {
    let base = "\(out)/\(icon.folder)"
    for layer in ["Back", "Front"] {
        var images: [[String: Any]] = []
        for s in icon.sizes {
            let c = context(s.w, s.h)
            let w = CGFloat(s.w), h = CGFloat(s.h)
            if layer == "Back" {
                drawBack(c, w, h, glowAt: CGPoint(x: w * 0.5, y: h * 0.45))
            } else {
                drawPlay(c, in: CGRect(x: 0, y: -h * 0.09, width: w, height: h * 1.18))
            }
            save(c, "\(base)/\(layer).imagestacklayer/Content.imageset/\(s.name).png")
            images.append(["idiom": "tv", "filename": "\(s.name).png", "scale": "\(s.scale)x"])
        }
        let json: [String: Any] = ["images": images, "info": ["author": "xcode", "version": 1]]
        try! JSONSerialization.data(withJSONObject: json, options: .prettyPrinted)
            .write(to: URL(fileURLWithPath: "\(base)/\(layer).imagestacklayer/Content.imageset/Contents.json"))
        try! #"{"info":{"author":"xcode","version":1}}"#.write(toFile: "\(base)/\(layer).imagestacklayer/Contents.json", atomically: true, encoding: .utf8)
    }
    try! #"{"info":{"author":"xcode","version":1},"layers":[{"filename":"Front.imagestacklayer"},{"filename":"Back.imagestacklayer"}]}"#
        .write(toFile: "\(base)/Contents.json", atomically: true, encoding: .utf8)
}

// MARK: Top shelf: the backdrop, the play mark and the wordmark.

let shelves: [(folder: String, w: Int, h: Int)] = [("Top Shelf Image.imageset", 1920, 720), ("Top Shelf Image Wide.imageset", 2320, 720)]
for shelf in shelves {
    var images: [[String: Any]] = []
    for scale in [1, 2] {
        let w = shelf.w * scale, h = shelf.h * scale
        let c = context(w, h)
        let W = CGFloat(w), H = CGFloat(h)
        drawBack(c, W, H, glowAt: CGPoint(x: W * 0.5, y: H * 0.5))
        let markH = H * 0.42
        let markW = markH * 0.6
        let textSize = H * 0.24
        let gap = H * 0.04
        let textW = textSize * 2.45
        let x0 = (W - (markW + gap + textW)) / 2
        drawPlay(c, in: CGRect(x: x0 - markH * 0.07, y: (H - markH) / 2, width: markW, height: markH))
        drawWordmark(c, "plaid", at: CGPoint(x: x0 + markW + gap, y: H / 2 + textSize * 0.36), size: textSize)
        let name = scale == 1 ? "shelf.png" : "shelf@2x.png"
        save(c, "\(out)/\(shelf.folder)/\(name)")
        images.append(["idiom": "tv", "filename": name, "scale": "\(scale)x"])
    }
    let json: [String: Any] = ["images": images, "info": ["author": "xcode", "version": 1]]
    try! JSONSerialization.data(withJSONObject: json, options: .prettyPrinted)
        .write(to: URL(fileURLWithPath: "\(out)/\(shelf.folder)/Contents.json"))
}

let brand: [String: Any] = [
    "assets": [
        ["filename": "App Icon - App Store.imagestack", "idiom": "tv", "role": "primary-app-icon", "size": "1280x768"],
        ["filename": "App Icon.imagestack", "idiom": "tv", "role": "primary-app-icon", "size": "400x240"],
        ["filename": "Top Shelf Image Wide.imageset", "idiom": "tv", "role": "top-shelf-image-wide", "size": "2320x720"],
        ["filename": "Top Shelf Image.imageset", "idiom": "tv", "role": "top-shelf-image", "size": "1920x720"],
    ],
    "info": ["author": "xcode", "version": 1],
]
try! JSONSerialization.data(withJSONObject: brand, options: .prettyPrinted).write(to: URL(fileURLWithPath: "\(out)/Contents.json"))
print("icons written to \(out)")
