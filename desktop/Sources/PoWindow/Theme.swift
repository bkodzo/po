import AppKit

/// A theme exported by `po theme export`: colours and looping animations as
/// plain data, so this window needs no drawing code of its own.
struct ThemePack: Decodable {
    struct Animation: Decodable {
        let w: Int
        let h: Int
        let frames: [String]
        let sequence: [Int]
    }

    struct MoodPack: Decodable {
        let palette: [String: [Int]]
        let full: Animation
        let mini: Animation
    }

    let version: Int
    let name: String
    let fps: [String: Int]
    let moods: [String: MoodPack]
    let lines: [String: [String]]

    static let supportedVersion = 1

    static func load(from url: URL) throws -> ThemePack {
        let pack = try JSONDecoder().decode(ThemePack.self, from: Data(contentsOf: url))
        guard pack.version == supportedVersion else {
            throw NSError(domain: "Po", code: 1, userInfo: [NSLocalizedDescriptionKey:
                "theme version \(pack.version) is not supported (this window understands \(supportedVersion))"])
        }
        return pack
    }
}

/// Turns the pack's pixel letters into images, and remembers them.
final class FrameCache {
    let pack: ThemePack
    private var images: [String: CGImage] = [:]

    init(pack: ThemePack) { self.pack = pack }

    func animation(for mood: String) -> ThemePack.Animation? {
        (pack.moods[mood] ?? pack.moods["calm"])?.full
    }

    /// The picture to show at a given tick of a mood's loop.
    func image(mood: String, tick: Int) -> CGImage? {
        guard let moodPack = pack.moods[mood] ?? pack.moods["calm"],
              !moodPack.full.sequence.isEmpty else { return nil }
        let anim = moodPack.full
        let index = anim.sequence[tick % anim.sequence.count]
        let key = "\(mood)/\(index)"
        if let cached = images[key] { return cached }
        guard let image = build(frame: anim.frames[index], width: anim.w, height: anim.h, palette: moodPack.palette) else {
            return nil
        }
        images[key] = image
        return image
    }

    /// One bitmap pixel per sprite pixel; scaling happens when drawing, with no
    /// smoothing, so the pixels stay sharp.
    private func build(frame: String, width: Int, height: Int, palette: [String: [Int]]) -> CGImage? {
        let letters = Array(frame)
        guard letters.count == width * height else { return nil }
        var bytes = [UInt8](repeating: 0, count: width * height * 4)
        for (i, ch) in letters.enumerated() {
            guard let c = palette[String(ch)], c.count == 3 else { continue }
            bytes[i * 4] = UInt8(clamping: c[0])
            bytes[i * 4 + 1] = UInt8(clamping: c[1])
            bytes[i * 4 + 2] = UInt8(clamping: c[2])
            bytes[i * 4 + 3] = 255
        }
        guard let provider = CGDataProvider(data: Data(bytes) as CFData) else { return nil }
        return CGImage(
            width: width, height: height, bitsPerComponent: 8, bitsPerPixel: 32,
            bytesPerRow: width * 4, space: CGColorSpace(name: CGColorSpace.sRGB)!,
            bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedLast.rawValue),
            provider: provider, decode: nil, shouldInterpolate: false, intent: .defaultIntent)
    }

    /// Whether the pixel at (x, y) of a frame is drawn rather than transparent.
    func isSolid(mood: String, tick: Int, x: Int, y: Int) -> Bool {
        guard let moodPack = pack.moods[mood] ?? pack.moods["calm"], !moodPack.full.sequence.isEmpty else { return false }
        let anim = moodPack.full
        guard x >= 0, y >= 0, x < anim.w, y < anim.h else { return false }
        let letters = Array(anim.frames[anim.sequence[tick % anim.sequence.count]])
        return moodPack.palette[String(letters[y * anim.w + x])] != nil
    }
}
