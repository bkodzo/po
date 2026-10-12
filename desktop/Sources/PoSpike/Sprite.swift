import AppKit

/// The creature, as a grid of palette letters. "." is transparent. For the
/// spike this is one still frame copied from the Go side; the real window will
/// play frames from a theme pack.
enum Sprite {
    static let rows: [String] = [
        "....................",
        "....................",
        "....................",
        "....................",
        "....................",
        "....................",
        "......oooooooo......",
        ".....obllbbbbbo.....",
        "....obllbbbbbbbo....",
        "...oblbbbbbbbbbbo...",
        "...obbbbbbbbbbbbo...",
        "...obbwkbbbbwkbbo...",
        "...obbkkbbbbkkbbo...",
        "...obbbbbbbbbbbbo...",
        "..boccbbbbbbbbccob..",
        "..bobbbbmbbmbbbbob..",
        "..oobbbbbmmbbbbboo..",
        "....osbbbbbbbbso....",
        "....osssssssssso....",
        ".....oooooooooo.....",
        "......fff..fff......",
        "......ooo..ooo......",
        "....................",
        "....hhhhhhhhhhhh....",
    ]

    static let palette: [Character: (UInt8, UInt8, UInt8)] = [
        "o": (60, 40, 50),
        "b": (245, 160, 70),
        "l": (255, 205, 130),
        "s": (205, 120, 50),
        "f": (190, 105, 45),
        "w": (255, 255, 255),
        "k": (40, 30, 40),
        "m": (150, 40, 50),
        "c": (255, 130, 130),
        "h": (90, 90, 100),
        "g": (225, 232, 240),
        "z": (150, 190, 255),
        "y": (255, 230, 90),
    ]

    static var width: Int { rows[0].count }
    static var height: Int { rows.count }

    /// True if the pixel at (x, y) is drawn rather than transparent.
    static func isOpaque(x: Int, y: Int) -> Bool {
        guard y >= 0, y < rows.count else { return false }
        let row = Array(rows[y])
        guard x >= 0, x < row.count else { return false }
        return palette[row[x]] != nil
    }

    /// One bitmap pixel per sprite pixel; scaling is done when drawing, with no
    /// smoothing, so the pixels stay sharp.
    static func image() -> NSImage {
        var bytes = [UInt8](repeating: 0, count: width * height * 4)
        for (y, line) in rows.enumerated() {
            for (x, ch) in line.enumerated() {
                guard let c = palette[ch] else { continue }
                let i = (y * width + x) * 4
                bytes[i] = c.0
                bytes[i + 1] = c.1
                bytes[i + 2] = c.2
                bytes[i + 3] = 255
            }
        }
        let provider = CGDataProvider(data: Data(bytes) as CFData)!
        let cg = CGImage(
            width: width, height: height, bitsPerComponent: 8, bitsPerPixel: 32,
            bytesPerRow: width * 4, space: CGColorSpace(name: CGColorSpace.sRGB)!,
            bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedLast.rawValue),
            provider: provider, decode: nil, shouldInterpolate: false, intent: .defaultIntent)!
        return NSImage(cgImage: cg, size: NSSize(width: width, height: height))
    }
}
