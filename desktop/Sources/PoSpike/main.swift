import AppKit
import CoreGraphics

// A spike: does a small transparent window behave the way a desktop pet needs
// to? It floats above other windows, never takes focus, lets clicks through
// where it is transparent, can be dragged, and snaps to screen edges.
//
//   PoSpike                    show the pet
//   PoSpike --level popup      try another window level (see windowLevel)
//   PoSpike --scale 4          pixels per sprite pixel
//   PoSpike --seconds 10       quit by itself after a while
//   PoSpike --probe            print display info and test reading other windows' positions

struct Options {
    var level = "status"
    var scale = 5
    var seconds: Double? = nil
    var probe = false
}

func parseOptions() -> Options {
    var o = Options()
    var args = Array(CommandLine.arguments.dropFirst())
    while !args.isEmpty {
        let a = args.removeFirst()
        switch a {
        case "--level": if !args.isEmpty { o.level = args.removeFirst() }
        case "--scale": if !args.isEmpty { o.scale = max(1, Int(args.removeFirst()) ?? 5) }
        case "--seconds": if !args.isEmpty { o.seconds = Double(args.removeFirst()) }
        case "--probe": o.probe = true
        default: break
        }
    }
    return o
}

func windowLevel(named name: String) -> NSWindow.Level {
    switch name {
    case "normal": return .normal
    case "floating": return .floating
    case "status": return .statusBar
    case "popup": return .popUpMenu
    case "screensaver": return .screenSaver
    case "shield": return NSWindow.Level(rawValue: Int(CGShieldingWindowLevel()))
    default: return .statusBar
    }
}

// MARK: - The view

final class PetView: NSView {
    let image = Sprite.image()
    let scale: Int
    var bob = 0
    var dragging = false
    private var grabOffset = NSPoint.zero

    init(scale: Int) {
        self.scale = scale
        let size = NSSize(width: Sprite.width * scale, height: Sprite.height * scale + 2 * scale)
        super.init(frame: NSRect(origin: .zero, size: size))
    }

    required init?(coder: NSCoder) { fatalError("not used") }

    override func draw(_ dirtyRect: NSRect) {
        NSGraphicsContext.current?.imageInterpolation = .none
        let rect = NSRect(x: 0, y: CGFloat(bob * scale), width: CGFloat(Sprite.width * scale), height: CGFloat(Sprite.height * scale))
        image.draw(in: rect, from: .zero, operation: .sourceOver, fraction: 1)
    }

    /// Whether a point in this view lands on a drawn sprite pixel.
    func isSolid(at point: NSPoint) -> Bool {
        let x = Int(point.x) / scale
        let fromBottom = Int((point.y - CGFloat(bob * scale)) / CGFloat(scale))
        let y = Sprite.height - 1 - fromBottom
        return point.x >= 0 && point.y >= 0 && Sprite.isOpaque(x: x, y: y)
    }

    override func mouseDown(with event: NSEvent) {
        dragging = true
        window?.ignoresMouseEvents = false
        grabOffset = event.locationInWindow
    }

    override func mouseDragged(with event: NSEvent) {
        guard let window = window else { return }
        let p = NSEvent.mouseLocation
        window.setFrameOrigin(NSPoint(x: p.x - grabOffset.x, y: p.y - grabOffset.y))
    }

    override func mouseUp(with event: NSEvent) {
        dragging = false
        snapToEdges()
    }

    override func rightMouseDown(with event: NSEvent) {
        let menu = NSMenu()
        menu.addItem(withTitle: "Quit Po spike", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "")
        NSMenu.popUpContextMenu(menu, with: event, for: self)
    }

    /// Pull the window to a screen edge or corner when it is dropped close to one.
    func snapToEdges() {
        guard let window = window, let screen = window.screen ?? NSScreen.main else { return }
        let area = screen.visibleFrame
        let margin: CGFloat = 12
        let reach: CGFloat = 32
        var f = window.frame
        if abs(f.minX - area.minX) < reach { f.origin.x = area.minX + margin }
        if abs(area.maxX - f.maxX) < reach { f.origin.x = area.maxX - f.width - margin }
        if abs(f.minY - area.minY) < reach { f.origin.y = area.minY + margin }
        if abs(area.maxY - f.maxY) < reach { f.origin.y = area.maxY - f.height - margin }
        window.setFrameOrigin(f.origin)
    }
}

// MARK: - The app

final class AppDelegate: NSObject, NSApplicationDelegate {
    let options: Options
    var panel: NSPanel!
    var view: PetView!
    var timer: Timer?

    init(options: Options) { self.options = options }

    func applicationDidFinishLaunching(_ notification: Notification) {
        view = PetView(scale: options.scale)
        panel = NSPanel(
            contentRect: NSRect(origin: .zero, size: view.frame.size),
            styleMask: [.borderless, .nonactivatingPanel],
            backing: .buffered, defer: false)
        panel.contentView = view
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.hasShadow = false
        panel.isFloatingPanel = true // this resets the level, so set the level after it
        panel.level = windowLevel(named: options.level)
        panel.hidesOnDeactivate = false
        panel.worksWhenModal = true
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .stationary, .ignoresCycle]
        panel.acceptsMouseMovedEvents = true
        panel.ignoresMouseEvents = true // until the pointer is over the sprite

        if let area = NSScreen.main?.visibleFrame {
            panel.setFrameOrigin(NSPoint(x: area.maxX - view.frame.width - 24, y: area.minY + 24))
        }
        panel.orderFrontRegardless()

        // Clicks pass through the transparent parts. Mouse-moved events reach us
        // even while the window ignores the mouse, through the global monitor.
        NSEvent.addGlobalMonitorForEvents(matching: [.mouseMoved]) { [weak self] _ in self?.updateClickThrough() }
        NSEvent.addLocalMonitorForEvents(matching: [.mouseMoved]) { [weak self] event in
            self?.updateClickThrough()
            return event
        }

        timer = Timer.scheduledTimer(withTimeInterval: 0.25, repeats: true) { [weak self] _ in
            guard let self = self else { return }
            self.view.bob = 1 - self.view.bob
            self.view.needsDisplay = true
        }

        print("Po spike is showing: level \(options.level) (\(panel.level.rawValue)), scale \(options.scale), \(Int(view.frame.width))x\(Int(view.frame.height)) points")
        if let seconds = options.seconds {
            DispatchQueue.main.asyncAfter(deadline: .now() + seconds) {
                print("done after \(seconds)s")
                NSApp.terminate(nil)
            }
        }
    }

    func updateClickThrough() {
        guard !view.dragging else { return }
        let p = NSEvent.mouseLocation
        var solid = false
        if panel.frame.contains(p) {
            solid = view.isSolid(at: view.convert(panel.convertPoint(fromScreen: p), from: nil))
        }
        panel.ignoresMouseEvents = !solid
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { false }
}

// MARK: - Probe

/// Reports what can be learned about other windows without any permission.
func probe() {
    _ = NSApplication.shared
    print("macOS \(ProcessInfo.processInfo.operatingSystemVersionString)")
    for (i, s) in NSScreen.screens.enumerated() {
        print("display \(i): frame \(s.frame), usable \(s.visibleFrame), scale \(s.backingScaleFactor)x")
    }
    print("screen recording permission for this program: \(CGPreflightScreenCaptureAccess())")

    let raw = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
    let normal = raw.filter { ($0[kCGWindowLayer as String] as? Int) == 0 }
    let withBounds = normal.filter { $0[kCGWindowBounds as String] != nil }
    let withNames = normal.filter { !(($0[kCGWindowName as String] as? String) ?? "").isEmpty }
    print("on-screen windows: \(raw.count) total, \(normal.count) ordinary")
    print("  with a position and size: \(withBounds.count)")
    print("  with a title:             \(withNames.count)")
    // Our own window may sit on any layer, so look for it across all of them.
    for w in raw where ((w[kCGWindowOwnerName as String] as? String) ?? "") == "PoSpike" {
        let b = (w[kCGWindowBounds as String] as? [String: Any]) ?? [:]
        print("  PoSpike window: layer \(w[kCGWindowLayer as String] ?? "?"), alpha \(w[kCGWindowAlpha as String] ?? "?"), x \(b["X"] ?? "?") y \(b["Y"] ?? "?") w \(b["Width"] ?? "?") h \(b["Height"] ?? "?")")
    }
    for w in normal.prefix(8) {
        let owner = (w[kCGWindowOwnerName as String] as? String) ?? "?"
        let bounds = (w[kCGWindowBounds as String] as? [String: Any]).map { b in
            "x \(b["X"] ?? "?") y \(b["Y"] ?? "?") w \(b["Width"] ?? "?") h \(b["Height"] ?? "?")"
        } ?? "no bounds"
        let name = (w[kCGWindowName as String] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? "(no title)"
        print("  \(owner): \(bounds) | \(name)")
    }
}

// MARK: - Main

let options = parseOptions()
if options.probe {
    probe()
} else {
    let app = NSApplication.shared
    app.setActivationPolicy(.accessory)
    let delegate = AppDelegate(options: options)
    app.delegate = delegate
    app.run()
}
