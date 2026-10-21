import AppKit
import CoreGraphics

// Po's desktop window: a small transparent pet that floats above other windows,
// never takes focus, lets clicks through where it is transparent, can be dragged
// and snaps to screen edges. It owns no logic: it plays the theme Po exported
// and follows the state Po streams.
//
//   PoWindow                  show the pet (Po must be running)
//   PoWindow --level popup    try another window level
//   PoWindow --scale 4        pixels per sprite pixel
//   PoWindow --seconds 10     quit by itself after a while
//   PoWindow --listen         no window: print what Po streams, to check the link
//   PoWindow --log            print the current mood and frame once a second
//   PoWindow --probe          print display info and what is readable about other windows

struct Options {
    var level = "status"
    var scale = 5
    var seconds: Double? = nil
    var probe = false
    var listen = false
    var log = false
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
        case "--listen": o.listen = true
        case "--log": o.log = true
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
    var cache: FrameCache?
    var mood = "calm"
    var tick = 0
    var line: String?
    var lineShownAt = Date.distantPast
    var dragging = false
    var onDragEnd: (() -> Void)?

    let scale: Int
    private var grabOffset = NSPoint.zero

    static let bubbleRoom: CGFloat = 64
    static let minWidth: CGFloat = 260
    static let lineLife: TimeInterval = 4

    init(scale: Int) {
        self.scale = scale
        super.init(frame: NSRect(origin: .zero, size: PetView.size(scale: scale, sprite: (20, 24))))
    }

    required init?(coder: NSCoder) { fatalError("not used") }

    static func size(scale: Int, sprite: (Int, Int)) -> NSSize {
        NSSize(width: max(minWidth, CGFloat(sprite.0 * scale)), height: CGFloat(sprite.1 * scale) + bubbleRoom)
    }

    var spriteSize: (Int, Int) {
        guard let anim = cache?.animation(for: "calm") else { return (20, 24) }
        return (anim.w, anim.h)
    }

    /// Where the creature is drawn: the bottom-right of the view, so the speech
    /// bubble has room above and to its left.
    var petRect: NSRect {
        let (w, h) = spriteSize
        return NSRect(x: bounds.width - CGFloat(w * scale), y: 0, width: CGFloat(w * scale), height: CGFloat(h * scale))
    }

    override func draw(_ dirtyRect: NSRect) {
        guard let ctx = NSGraphicsContext.current?.cgContext,
              let image = cache?.image(mood: mood, tick: tick) else { return }
        ctx.interpolationQuality = .none
        ctx.draw(image, in: petRect)
        drawBubble()
    }

    private func drawBubble() {
        guard let line = line, !line.isEmpty else { return }
        let age = Date().timeIntervalSince(lineShownAt)
        guard age < PetView.lineLife else { return }
        let alpha = CGFloat(age > PetView.lineLife - 0.6 ? (PetView.lineLife - age) / 0.6 : 1)

        let text = NSAttributedString(string: line, attributes: [
            .font: NSFont.systemFont(ofSize: 12, weight: .medium),
            .foregroundColor: NSColor(white: 0.1, alpha: alpha),
        ])
        let measured = text.boundingRect(with: NSSize(width: 170, height: 200), options: [.usesLineFragmentOrigin])
        let pad: CGFloat = 8
        let box = NSRect(
            x: max(4, bounds.width - ceil(measured.width) - 2 * pad - 6),
            y: petRect.maxY + 4,
            width: ceil(measured.width) + 2 * pad,
            height: ceil(measured.height) + 2 * pad)
        let path = NSBezierPath(roundedRect: box, xRadius: 8, yRadius: 8)
        NSColor(white: 1, alpha: 0.95 * alpha).setFill()
        path.fill()
        NSColor(white: 0.3, alpha: alpha).setStroke()
        path.lineWidth = 1
        path.stroke()
        text.draw(with: box.insetBy(dx: pad, dy: pad), options: [.usesLineFragmentOrigin])
    }

    /// Whether a point in this view lands on a drawn pixel of the creature.
    func isSolid(at point: NSPoint) -> Bool {
        let rect = petRect
        guard rect.contains(point) else { return false }
        let x = Int((point.x - rect.minX) / CGFloat(scale))
        let fromBottom = Int((point.y - rect.minY) / CGFloat(scale))
        let (_, h) = spriteSize
        return cache?.isSolid(mood: mood, tick: tick, x: x, y: h - 1 - fromBottom) ?? false
    }

    // Dragging: the window follows the pointer, and snaps to an edge on release.

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
        onDragEnd?()
    }

    override func rightMouseDown(with event: NSEvent) {
        let menu = NSMenu()
        menu.addItem(withTitle: "Quit Po window", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "")
        NSMenu.popUpContextMenu(menu, with: event, for: self)
    }

    /// The creature, not the whole transparent window, is what snaps to an edge.
    func snapToEdges() {
        guard let window = window, let screen = window.screen ?? NSScreen.main else { return }
        let area = screen.visibleFrame
        let margin: CGFloat = 12
        let reach: CGFloat = 32
        let pet = petRect
        var origin = window.frame.origin
        let petLeft = origin.x + pet.minX, petRight = origin.x + pet.maxX
        let petBottom = origin.y + pet.minY, petTop = origin.y + pet.maxY
        if abs(petLeft - area.minX) < reach { origin.x += area.minX + margin - petLeft }
        if abs(area.maxX - petRight) < reach { origin.x += area.maxX - margin - petRight }
        if abs(petBottom - area.minY) < reach { origin.y += area.minY + margin - petBottom }
        if abs(area.maxY - petTop) < reach { origin.y += area.maxY - margin - petTop }
        window.setFrameOrigin(origin)
    }
}

// MARK: - The app

final class AppDelegate: NSObject, NSApplicationDelegate {
    let options: Options
    var panel: NSPanel!
    var view: PetView!
    let stream = StateStream()
    var seenLineSeq: Int?
    var lastStatus = ""
    var lastFrameKey = ""
    // Counters for --log, to see where the time goes.
    var ticksSinceLog = 0, redrawsSinceLog = 0, mouseSinceLog = 0

    init(options: Options) { self.options = options }

    func applicationDidFinishLaunching(_ notification: Notification) {
        view = PetView(scale: options.scale)
        view.onDragEnd = { [weak self] in self?.savePosition() }

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
        panel.ignoresMouseEvents = true // until the pointer is over the creature
        restorePosition()

        // Clicks pass through everything but the creature. Mouse-moved events
        // reach us even while the window ignores the mouse, via the global monitor.
        NSEvent.addGlobalMonitorForEvents(matching: [.mouseMoved]) { [weak self] _ in self?.updateClickThrough() }
        NSEvent.addLocalMonitorForEvents(matching: [.mouseMoved]) { [weak self] event in
            self?.updateClickThrough()
            return event
        }

        stream.onStatus = { [weak self] status in self?.handle(status: status) }
        stream.onState = { [weak self] state in self?.apply(state) }
        stream.start()
        if loadTheme() { panel.orderFrontRegardless() }

        // One tick per frame of the animation. The theme says how fast that is.
        let fps = max(1, view.cache?.pack.fps["full"] ?? 8)
        Timer.scheduledTimer(withTimeInterval: 1.0 / Double(fps), repeats: true) { [weak self] _ in self?.advance() }
        if options.log {
            Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in self?.logFrame() }
        }

        if let seconds = options.seconds {
            DispatchQueue.main.asyncAfter(deadline: .now() + seconds) {
                print("done after \(seconds)s")
                NSApp.terminate(nil)
            }
        }
        print("Po window started: level \(options.level) (\(panel.level.rawValue)), scale \(options.scale)")
    }

    /// Reads the theme the pet exported. Returns whether there is one to play.
    @discardableResult
    func loadTheme() -> Bool {
        do {
            let pack = try ThemePack.load(from: PoPaths.theme)
            view.cache = FrameCache(pack: pack)
            let size = PetView.size(scale: options.scale, sprite: view.spriteSize)
            let oldTop = panel.frame.maxY
            let oldRight = panel.frame.maxX
            view.setFrameSize(size)
            panel.setContentSize(size)
            panel.setFrameOrigin(NSPoint(x: oldRight - size.width, y: oldTop - size.height))
            view.needsDisplay = true
            return true
        } catch {
            if lastStatus != "no theme" {
                lastStatus = "no theme"
                print("no theme yet (\(error.localizedDescription)); Po writes it when it starts")
            }
            return false
        }
    }

    func handle(status: String) {
        if status != lastStatus { print("po: \(status)") }
        lastStatus = status
        switch status {
        case "connected":
            seenLineSeq = nil // the first state after connecting is not news
            if loadTheme() { panel.orderFrontRegardless() }
        case "disconnected":
            break
        default:
            break
        }
    }

    func apply(_ state: PetState) {
        if state.mood != view.mood {
            view.mood = state.mood
            view.tick = 0
        }
        if let seen = seenLineSeq {
            if state.lineSeq > seen, let line = state.line, !line.isEmpty {
                view.line = line
                view.lineShownAt = Date()
            }
        }
        seenLineSeq = state.lineSeq
        view.needsDisplay = true
    }

    /// Moves the animation on one tick, and only redraws if what is on screen
    /// would change: a new picture, or a speech bubble that is showing or fading.
    func advance() {
        guard let cache = view.cache, let anim = cache.animation(for: view.mood), !anim.sequence.isEmpty else { return }
        view.tick += 1
        ticksSinceLog += 1
        let key = "\(view.mood)/\(anim.sequence[view.tick % anim.sequence.count])"
        let bubbleActive = view.line != nil && Date().timeIntervalSince(view.lineShownAt) < PetView.lineLife + 0.3
        if key != lastFrameKey || bubbleActive {
            lastFrameKey = key
            view.needsDisplay = true
            redrawsSinceLog += 1
        }
    }

    func logFrame() {
        guard let cache = view.cache, let anim = cache.animation(for: view.mood), !anim.sequence.isEmpty else {
            print("log: no theme loaded")
            return
        }
        let index = anim.sequence[view.tick % anim.sequence.count]
        print("log: mood \(view.mood), tick \(view.tick), frame \(index) of \(anim.frames.count)\(view.line.map { ", line \"\($0)\"" } ?? "") | last second: \(ticksSinceLog) ticks, \(redrawsSinceLog) redraws, \(mouseSinceLog) mouse events")
        ticksSinceLog = 0; redrawsSinceLog = 0; mouseSinceLog = 0
    }

    func updateClickThrough() {
        mouseSinceLog += 1
        guard !view.dragging else { return }
        let p = NSEvent.mouseLocation
        var solid = false
        if panel.frame.contains(p) {
            solid = view.isSolid(at: view.convert(panel.convertPoint(fromScreen: p), from: nil))
        }
        panel.ignoresMouseEvents = !solid
    }

    // The window remembers where it was left.

    func savePosition() {
        let o = panel.frame.origin
        UserDefaults.standard.set("\(o.x),\(o.y)", forKey: "origin")
    }

    func restorePosition() {
        if let saved = UserDefaults.standard.string(forKey: "origin") {
            let parts = saved.split(separator: ",").compactMap { Double($0) }
            if parts.count == 2 {
                let origin = NSPoint(x: parts[0], y: parts[1])
                let probe = NSRect(origin: origin, size: panel.frame.size)
                if NSScreen.screens.contains(where: { $0.visibleFrame.intersects(probe) }) {
                    panel.setFrameOrigin(origin)
                    return
                }
            }
        }
        if let area = NSScreen.main?.visibleFrame {
            panel.setFrameOrigin(NSPoint(x: area.maxX - panel.frame.width - 12, y: area.minY + 12))
        }
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { false }
}

// MARK: - Listen and probe (no window)

/// Prints what Po streams, to check the link without drawing anything.
func listen(seconds: Double) {
    if let pack = try? ThemePack.load(from: PoPaths.theme) {
        let calm = pack.moods["calm"]?.full
        print("theme: \(pack.name), version \(pack.version), moods \(pack.moods.keys.sorted()), calm loop \(calm?.sequence.count ?? 0) ticks of \(calm?.frames.count ?? 0) frames")
    } else {
        print("theme: none found at \(PoPaths.theme.path)")
    }
    let stream = StateStream()
    stream.onStatus = { print("status: \($0)") }
    stream.onState = { s in print("state: mood \(s.mood), resting \(s.resting), line_seq \(s.lineSeq)\(s.line.map { ", line \"\($0)\"" } ?? "")") }
    stream.start()
    DispatchQueue.main.asyncAfter(deadline: .now() + seconds) { exit(0) }
    dispatchMain()
}

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
    print("on-screen windows: \(raw.count) total, \(normal.count) ordinary")
    // Our own window may sit on any layer, so look for it across all of them.
    for w in raw where ((w[kCGWindowOwnerName as String] as? String) ?? "") == "PoWindow" {
        let b = (w[kCGWindowBounds as String] as? [String: Any]) ?? [:]
        print("  PoWindow window: layer \(w[kCGWindowLayer as String] ?? "?"), alpha \(w[kCGWindowAlpha as String] ?? "?"), x \(b["X"] ?? "?") y \(b["Y"] ?? "?") w \(b["Width"] ?? "?") h \(b["Height"] ?? "?")")
    }
    for w in normal.prefix(8) {
        let owner = (w[kCGWindowOwnerName as String] as? String) ?? "?"
        let bounds = (w[kCGWindowBounds as String] as? [String: Any]).map { b in
            "x \(b["X"] ?? "?") y \(b["Y"] ?? "?") w \(b["Width"] ?? "?") h \(b["Height"] ?? "?")"
        } ?? "no bounds"
        print("  \(owner): \(bounds)")
    }
}

// MARK: - Main

let options = parseOptions()

/// Ctrl+Z would suspend the program and leave its window frozen on screen, with
/// nothing left running to remove it. Quit instead.
var suspendWatcher: DispatchSourceSignal?
func quitOnSuspend() {
    signal(SIGTSTP, SIG_IGN)
    let watcher = DispatchSource.makeSignalSource(signal: SIGTSTP, queue: .main)
    watcher.setEventHandler {
        print("Ctrl+Z received: quitting instead of freezing the window")
        NSApp.terminate(nil)
    }
    watcher.resume()
    suspendWatcher = watcher
}

if options.probe {
    probe()
} else if options.listen {
    listen(seconds: options.seconds ?? 10)
} else {
    let app = NSApplication.shared
    app.setActivationPolicy(.accessory)
    quitOnSuspend()
    let delegate = AppDelegate(options: options)
    app.delegate = delegate
    app.run()
}
