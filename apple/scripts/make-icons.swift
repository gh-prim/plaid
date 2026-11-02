// Draws the tvOS app icon layers, the top shelf images and the launch color
// from the V2 logo (board "Logo · Woven play, splash"): the woven play
// button, a play triangle crossed by one warp and one weft thread, cream
// #f4f0e9 on graphite #1c1c1e, and the "plaid" lettering.
// Run from apple/: swift scripts/make-icons.swift
import AppKit
import CoreGraphics
import ImageIO
import UniformTypeIdentifiers

let assets = "PLAID/Resources/Assets.xcassets"
let out = "\(assets)/App Icon & Top Shelf Image.brandassets"

let cream = CGColor(srgbRed: 0xf4 / 255, green: 0xf0 / 255, blue: 0xe9 / 255, alpha: 1)
let graphite = CGColor(srgbRed: 0x1c / 255, green: 0x1c / 255, blue: 0x1e / 255, alpha: 1)

/// `opaque`: no alpha channel, as the App Store wants for an iOS icon.
func context(_ w: Int, _ h: Int, opaque: Bool = false) -> CGContext {
    let c = CGContext(data: nil, width: w, height: h, bitsPerComponent: 8, bytesPerRow: 0,
                      space: CGColorSpace(name: CGColorSpace.sRGB)!,
                      bitmapInfo: (opaque ? CGImageAlphaInfo.noneSkipLast : .premultipliedLast).rawValue)!
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

func writeJSON(_ object: Any, _ path: String) {
    try! JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys])
        .write(to: URL(fileURLWithPath: path))
}

/// The woven play button in its 100-unit box: the rounded triangle, with the
/// two threads (x 46…51 and y 47.5…52.5) cut out so the ground shows through.
func drawMark(_ c: CGContext, origin: CGPoint, scale s: CGFloat) {
    c.saveGState()
    c.translateBy(x: origin.x, y: origin.y)
    c.scaleBy(x: s, y: s)
    let tri = CGMutablePath()
    tri.move(to: CGPoint(x: 33, y: 22)); tri.addLine(to: CGPoint(x: 81, y: 50)); tri.addLine(to: CGPoint(x: 33, y: 78)); tri.closeSubpath()
    // Clip to the four quarters around the threads, then fill and stroke.
    c.addRects([
        CGRect(x: -100, y: -100, width: 146, height: 147.5), CGRect(x: 51, y: -100, width: 149, height: 147.5),
        CGRect(x: 51, y: 52.5, width: 149, height: 147.5), CGRect(x: -100, y: 52.5, width: 146, height: 147.5),
    ])
    c.clip()
    c.setFillColor(cream); c.setStrokeColor(cream)
    c.addPath(tri); c.fillPath()
    c.addPath(tri); c.setLineWidth(6); c.setLineJoin(.round); c.strokePath()
    c.restoreGState()
}

/// The "plaid" lettering: circles and straight strokes of one weight, the i
/// dotted with a play button. Drawn in its viewBox units (baseline at y 100).
func drawLettering(_ c: CGContext, origin: CGPoint, scale s: CGFloat) {
    c.saveGState()
    c.translateBy(x: origin.x, y: origin.y)
    c.scaleBy(x: s, y: s)
    c.setStrokeColor(cream); c.setFillColor(cream)
    c.setLineWidth(12); c.setLineCap(.butt)
    for (cx, cy) in [(31.0, 75.0), (125, 75), (214, 75)] {
        c.addEllipse(in: CGRect(x: cx - 19, y: cy - 19, width: 38, height: 38))
    }
    for (x, y0, y1) in [(6.0, 56.0, 130.0), (144, 56, 100), (170, 56, 100), (233, 20, 100)] {
        c.move(to: CGPoint(x: x, y: y0)); c.addLine(to: CGPoint(x: x, y: y1))
    }
    c.move(to: CGPoint(x: 78, y: 20)); c.addLine(to: CGPoint(x: 78, y: 84))
    c.addQuadCurve(to: CGPoint(x: 94, y: 100), control: CGPoint(x: 78, y: 100))
    c.strokePath()
    let dot = CGMutablePath()
    dot.move(to: CGPoint(x: 164, y: 26)); dot.addLine(to: CGPoint(x: 180, y: 35)); dot.addLine(to: CGPoint(x: 164, y: 44)); dot.closeSubpath()
    c.addPath(dot); c.fillPath()
    c.addPath(dot); c.setLineWidth(3); c.setLineJoin(.round); c.strokePath()
    c.restoreGState()
}

// MARK: App icon: the graphite ground behind, the woven play button in front
// (its threads are transparent, so the ground shows through them).

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
                c.setFillColor(graphite); c.fill(CGRect(x: 0, y: 0, width: w, height: h))
            } else {
                // The 100-unit box spans the height, as on the board's square
                // icons; the triangle (x 33…81) sits a little right of center.
                let k = h / 100
                drawMark(c, origin: CGPoint(x: w / 2 - 55 * k, y: 0), scale: k)
            }
            save(c, "\(base)/\(layer).imagestacklayer/Content.imageset/\(s.name).png")
            images.append(["idiom": "tv", "filename": "\(s.name).png", "scale": "\(s.scale)x"])
        }
        writeJSON(["images": images, "info": ["author": "xcode", "version": 1]], "\(base)/\(layer).imagestacklayer/Content.imageset/Contents.json")
        writeJSON(["info": ["author": "xcode", "version": 1]], "\(base)/\(layer).imagestacklayer/Contents.json")
    }
    writeJSON(["info": ["author": "xcode", "version": 1], "layers": [["filename": "Front.imagestacklayer"], ["filename": "Back.imagestacklayer"]]], "\(base)/Contents.json")
}

// MARK: Top shelf: the lockup of the splash's last frame, centered on graphite.

let shelves: [(folder: String, w: Int, h: Int)] = [("Top Shelf Image.imageset", 1920, 720), ("Top Shelf Image Wide.imageset", 2320, 720)]
for shelf in shelves {
    var images: [[String: Any]] = []
    for scale in [1, 2] {
        let w = shelf.w * scale, h = shelf.h * scale
        let c = context(w, h)
        let W = CGFloat(w), H = CGFloat(h)
        c.setFillColor(graphite); c.fill(CGRect(x: 0, y: 0, width: W, height: H))
        // On the 1440 × 900 board the mark is at (500, 370) × 1.6 and the
        // lettering at (700, 395); the lockup spans x 553…943, y 405…531.
        let f = H * 0.34 / 126
        c.translateBy(x: W / 2 - 748 * f, y: H / 2 - 468 * f)
        c.scaleBy(x: f, y: f)
        drawMark(c, origin: CGPoint(x: 500, y: 370), scale: 1.6)
        drawLettering(c, origin: CGPoint(x: 700, y: 395), scale: 1)
        let name = scale == 1 ? "shelf.png" : "shelf@2x.png"
        save(c, "\(out)/\(shelf.folder)/\(name)")
        images.append(["idiom": "tv", "filename": name, "scale": "\(scale)x"])
    }
    writeJSON(["images": images, "info": ["author": "xcode", "version": 1]], "\(out)/\(shelf.folder)/Contents.json")
}

writeJSON([
    "assets": [
        ["filename": "App Icon - App Store.imagestack", "idiom": "tv", "role": "primary-app-icon", "size": "1280x768"],
        ["filename": "App Icon.imagestack", "idiom": "tv", "role": "primary-app-icon", "size": "400x240"],
        ["filename": "Top Shelf Image Wide.imageset", "idiom": "tv", "role": "top-shelf-image-wide", "size": "2320x720"],
        ["filename": "Top Shelf Image.imageset", "idiom": "tv", "role": "top-shelf-image", "size": "1920x720"],
    ],
    "info": ["author": "xcode", "version": 1],
], "\(out)/Contents.json")

// MARK: Launch screen: the splash's graphite, so the intro starts on the same color.

try? FileManager.default.createDirectory(atPath: "\(assets)/Splash.colorset", withIntermediateDirectories: true)
writeJSON([
    "colors": [["idiom": "universal", "color": ["color-space": "srgb", "components": ["red": "0x1C", "green": "0x1C", "blue": "0x1E", "alpha": "1.000"]]]],
    "info": ["author": "xcode", "version": 1],
], "\(assets)/Splash.colorset/Contents.json")

// MARK: iPhone and iPad: one 1024 square, the woven play button on graphite
// as on the board's square icon; the system rounds the corners.

let iosIcon = "iOS/Resources/Assets.xcassets/AppIcon.appiconset"
do {
    let c = context(1024, 1024, opaque: true)
    c.setFillColor(graphite); c.fill(CGRect(x: 0, y: 0, width: 1024, height: 1024))
    drawMark(c, origin: .zero, scale: 10.24)
    save(c, "\(iosIcon)/icon-1024.png")
    writeJSON([
        "images": [["idiom": "universal", "platform": "ios", "size": "1024x1024", "filename": "icon-1024.png"]],
        "info": ["author": "xcode", "version": 1],
    ], "\(iosIcon)/Contents.json")
    writeJSON(["info": ["author": "xcode", "version": 1]], "iOS/Resources/Assets.xcassets/Contents.json")
}

print("icons written to \(out) and \(iosIcon)")
