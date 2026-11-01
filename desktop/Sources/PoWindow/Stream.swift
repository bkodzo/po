import Foundation
import Darwin

/// What Po streams about the pet. See the daemon's State type.
struct PetState: Decodable, Equatable {
    let v: Int
    let mood: String
    let resting: String
    let line: String?
    let lineSeq: Int
    let theme: String
    /// An idle animation to play, or empty/nil. activitySeq goes up when a new one starts.
    let activity: String?
    let activitySeq: Int?

    enum CodingKeys: String, CodingKey {
        case v, mood, resting, line, theme, activity
        case lineSeq = "line_seq"
        case activitySeq = "activity_seq"
    }

    static let supportedVersion = 1
}

enum PoPaths {
    /// Where the pet listens, matching the Go side: PO_SOCK, or the config folder.
    static var socket: String {
        if let p = ProcessInfo.processInfo.environment["PO_SOCK"], !p.isEmpty { return p }
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        return support.appendingPathComponent("po/pet.sock").path
    }

    /// The theme the pet exported, next to its socket.
    static var theme: URL {
        URL(fileURLWithPath: socket).deletingLastPathComponent().appendingPathComponent("theme.json")
    }
}

/// Follows the pet's state stream, reconnecting whenever it drops. Everything
/// runs on a background thread and results come back on the main queue.
final class StateStream {
    var onState: ((PetState) -> Void)?
    var onStatus: ((String) -> Void)?
    private var running = true

    func start() {
        let thread = Thread { [weak self] in self?.loop() }
        thread.qualityOfService = .utility
        thread.start()
    }

    func stop() { running = false }

    private func report(_ text: String) {
        DispatchQueue.main.async { [weak self] in self?.onStatus?(text) }
    }

    private func loop() {
        var announcedWaiting = false
        while running {
            if let fd = connect() {
                announcedWaiting = false
                report("connected")
                read(from: fd)
                close(fd)
                report("disconnected")
            } else if !announcedWaiting {
                announcedWaiting = true
                report("waiting for Po (it starts at your next shell prompt, or run: po start)")
            }
            Thread.sleep(forTimeInterval: 2)
        }
    }

    private func connect() -> Int32? {
        guard let fd = connectToPo() else { return nil }
        let hello = Array("{\"op\":\"subscribe\"}\n".utf8)
        guard write(fd, hello, hello.count) == hello.count else { close(fd); return nil }
        return fd
    }

    private func read(from fd: Int32) {
        var buffer = Data()
        var chunk = [UInt8](repeating: 0, count: 4096)
        var first = true
        while running {
            let n = Darwin.read(fd, &chunk, chunk.count)
            if n <= 0 { return }
            buffer.append(contentsOf: chunk[0..<n])
            while let newline = buffer.firstIndex(of: 10) {
                var line = buffer[buffer.startIndex..<newline]
                buffer.removeSubrange(buffer.startIndex...newline)
                if first {
                    // The first line acknowledges the subscription: "ok {state}".
                    first = false
                    guard line.starts(with: Array("ok".utf8)) else { return }
                    line = line.dropFirst(2)
                }
                let text = Data(line).trimmed()
                guard !text.isEmpty, let state = try? JSONDecoder().decode(PetState.self, from: text) else { continue }
                guard state.v == PetState.supportedVersion else {
                    report("the pet speaks state version \(state.v); this window understands \(PetState.supportedVersion)")
                    return
                }
                DispatchQueue.main.async { [weak self] in self?.onState?(state) }
            }
        }
    }
}

private extension Data {
    func trimmed() -> Data {
        var d = self
        while let f = d.first, f == 32 || f == 13 || f == 9 { d.removeFirst() }
        while let l = d.last, l == 32 || l == 13 || l == 9 { d.removeLast() }
        return d
    }
}

/// Opens a connection to Po's socket, or returns nil if Po is not there.
func connectToPo() -> Int32? {
    let fd = socket(AF_UNIX, SOCK_STREAM, 0)
    guard fd >= 0 else { return nil }
    var addr = sockaddr_un()
    addr.sun_family = sa_family_t(AF_UNIX)
    let path = Array(PoPaths.socket.utf8CString)
    guard path.count <= MemoryLayout.size(ofValue: addr.sun_path) else { close(fd); return nil }
    withUnsafeMutablePointer(to: &addr.sun_path) { p in
        p.withMemoryRebound(to: CChar.self, capacity: path.count) { dst in
            for i in 0..<path.count { dst[i] = path[i] }
        }
    }
    let size = socklen_t(MemoryLayout<sockaddr_un>.size)
    let result = withUnsafePointer(to: &addr) { p in
        p.withMemoryRebound(to: sockaddr.self, capacity: 1) { Darwin.connect(fd, $0, size) }
    }
    if result != 0 { close(fd); return nil }
    return fd
}

/// Sends Po a single instruction, such as choosing a theme.
enum PoControl {
    static func send(op: String, name: String) {
        DispatchQueue.global(qos: .utility).async {
            guard let fd = connectToPo() else { return }
            defer { close(fd) }
            let line = Array("{\"op\":\"\(op)\",\"name\":\"\(name)\"}\n".utf8)
            guard write(fd, line, line.count) == line.count else { return }
            var buffer = [UInt8](repeating: 0, count: 64)
            var timeout = timeval(tv_sec: 1, tv_usec: 0)
            setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &timeout, socklen_t(MemoryLayout<timeval>.size))
            _ = Darwin.read(fd, &buffer, buffer.count) // wait for Po to acknowledge
        }
    }
}
